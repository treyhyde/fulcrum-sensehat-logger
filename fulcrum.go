package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/lanzafame/bobblehat/sense/screen"
	"github.com/lanzafame/bobblehat/sense/screen/color"
	"github.com/lanzafame/bobblehat/sense/stick"
	// "periph.io/x/periph/host"
)

const FormID = "5a7cffe6-05b2-4916-a510-16b30b869905"

type record struct {
	FormID     string            `json:"form_id"`
	Latitude   float64           `json:"latitude"`
	Longitude  float64           `json:"longitude"`
	Status     string            `json:"status"`
	FormValues map[string]string `json:"form_values"`
}

type apiPage struct {
	CurrentPage int      `json:"current_page"`
	TotalPages  int      `json:"total_pages"`
	TotalCount  int      `json:"total_count"`
	PerPage     int      `json:"per_page"`
	Records     []record `json:"records"`
}

type orientation struct {
	pitch float64
	roll  float64
	yaw   float64
}

func main() {

	ticker := time.NewTicker(2 * time.Second)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				updateDashboard()
			}
		}
	}()

	bobble()
}

func updateDashboard() {
	resp := fetchRecords()

	screen.Clear()
	fb := screen.NewFrameBuffer()

	for i, r := range resp.Records {
		x := i / 8
		y := i % 8

		switch r.Status {
		case "hot":
			fb.SetPixel(x, y, color.New(255, 0, 0))
		case "toasty":
			fb.SetPixel(x, y, color.New(255, 165, 0))
		case "warm":
			fb.SetPixel(x, y, color.New(255, 255, 0))
		case "cold":
			fb.SetPixel(x, y, color.New(135, 206, 250))
		case "frozen":
			fb.SetPixel(x, y, color.New(0, 0, 255))
		default:
			fb.SetPixel(x, y, color.New(0, 0, 0))
		}
		// fmt.Println(r.Status)
		screen.Draw(fb)
	}
}

func newRecord(status string, temp float64, humidity float64, orientation orientation, pressure float64) record {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))

	negLat := random.Int()%2 > 0
	negLong := random.Int()%2 > 0

	r := record{}
	r.FormID = FormID

	r.Latitude = random.Float64() * 90
	if negLat {
		r.Latitude = r.Latitude * -1
	}

	r.Longitude = random.Float64() * 180 // 180
	if negLong {
		r.Longitude = r.Longitude * -1
	}

	r.Status = status

	// 4834 orientation

	r.FormValues = map[string]string{
		"ebf0": strconv.FormatFloat(temp, 'f', -1, 64),
		"0a86": strconv.FormatFloat(humidity, 'f', -1, 64),
		"e633": strconv.FormatFloat(pressure, 'f', -1, 64),
		"840e": strconv.FormatFloat(orientation.roll, 'f', -1, 64),
		"1611": strconv.FormatFloat(orientation.pitch, 'f', -1, 64),
		"2987": strconv.FormatFloat(orientation.yaw, 'f', -1, 64),
	}

	return r
}

func fulcrumCreate(record record) {

	// Create a Resty Client
	client := resty.New()
	// POST Struct, default is JSON content type. No need to set one
	resp, err := client.R().
		SetBody(record).
		SetHeader("X-ApiToken", os.Getenv("FULCRUM_TOKEN")).
		// SetResult(&AuthSuccess{}). // or SetResult(AuthSuccess{}).
		// SetError(&AuthError{}).    // or SetError(AuthError{}).
		Post("https://api.fulcrumapp.com/api/v2/records.json")

	// Explore response object
	fmt.Println("Response Info:")
	fmt.Println("  Error      :", err)
	fmt.Println("  Status Code:", resp.StatusCode())
	fmt.Println("  Status     :", resp.Status())
	fmt.Println("  Proto      :", resp.Proto())
	fmt.Println("  Time       :", resp.Time())
	fmt.Println("  Received At:", resp.ReceivedAt())
	fmt.Println("  Body       :\n", resp)
	fmt.Println()

	//https://api.fulcrumapp.com/api/v2/records.json

	// newest_first=true
	// form_id
	// page = 1
	// per_page = 64
}

func fetchRecords() apiPage {
	client := resty.New()
	resp, err := client.R().
		SetHeader("X-ApiToken", os.Getenv("FULCRUM_TOKEN")).
		SetQueryParams(map[string]string{
			"newest_first": "true",
			"page":         "1",
			"per_page":     "64",
			"form_id":      FormID,
		}).
		SetResult(&apiPage{}).
		Get("https://api.fulcrumapp.com/api/v2/records.json")

	// Explore response object
	// fmt.Println("Response Info:")
	// fmt.Println("  Error      :", err)
	fmt.Println(" FetchRecentRecords:", resp.Status())
	if err != nil {
		fmt.Println("  Error      :", err)
	}
	// fmt.Println("  Status     :", resp.Status())
	// fmt.Println("  Proto      :", resp.Proto())
	// fmt.Println("  Time       :", resp.Time())
	// fmt.Println("  Received At:", resp.ReceivedAt())
	// fmt.Println("  Body       :\n", resp)
	// fmt.Println()
	return *resp.Result().(*apiPage)
}

func bobble() {
	var path string

	flag.StringVar(&path, "path", "/dev/input/event0", "path to the event device")

	// Parse command line flags
	flag.Parse()

	// Open the input device (and defer closing it)
	input, err := stick.Open(path)
	if err != nil {
		fmt.Printf("Unable to open input device: %s\nError: %v\n", path, err)
		os.Exit(1)
	}

	// Print the name of the input device
	fmt.Println(input.Name())

	// Set up a signals channel (stop the loop using Ctrl-C)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)

	// Loop forever
	for {
		select {
		case <-signals:
			fmt.Println("")

			// Exit the loop
			return
		case e := <-input.Events:
			temp := getTemp()
			switch e.Code {
			case stick.Enter:
				fmt.Println("⏎")
				go fulcrumCreate(newRecord("frozen", temp, getHumidity(), getOrientation(), getPressure()))
			case stick.Up:
				fmt.Println("↑")
				go fulcrumCreate(newRecord("hot", getTemp(), getHumidity(), getOrientation(), getPressure()))
			case stick.Down:
				fmt.Println("↓")
				go fulcrumCreate(newRecord("cold", getTemp(), getHumidity(), getOrientation(), getPressure()))
			case stick.Left:
				fmt.Println("←")
				go fulcrumCreate(newRecord("warm", getTemp(), getHumidity(), getOrientation(), getPressure()))
			case stick.Right:
				fmt.Println("→")
				go fulcrumCreate(newRecord("toasty", getTemp(), getHumidity(), getOrientation(), getPressure()))
			}

		}
	}
}

func getTemp() float64 {
	return senseHat("temperature")
}

func getHumidity() float64 {
	return senseHat("humidity")
}

func getPressure() float64 {
	return senseHat("pressure")
}

func getOrientation() orientation {
	out, err := exec.Command("python", "-c", `from sense_hat import SenseHat;o=SenseHat().get_orientation();print("\n".join(map(str, [o["pitch"],o["roll"],o["yaw"]])))`).Output()
	if err != nil {
		log.Fatal("orientation bombed ", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	pitch, _ := strconv.ParseFloat(lines[0], 64)
	roll, _ := strconv.ParseFloat(lines[1], 64)
	yaw, _ := strconv.ParseFloat(lines[2], 64)

	return orientation{pitch, roll, yaw}
}

func senseHat(function string) float64 {
	out, err := exec.Command("python", "-c", "from sense_hat import SenseHat;print(SenseHat().get_"+function+"())").Output()
	if err != nil {
		log.Fatal(err)
	}

	f, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		log.Fatal("Couldn't pare float for ", function, " ", err)
	}

	return f
}
