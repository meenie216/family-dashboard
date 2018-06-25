package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

type cals struct {
	Calendars []cal `json:"calendars"`
}

type cal struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type familyCalendar struct {
	MemberCalendars []memberCalendar `json:"memberCalendars"`
}

type memberCalendar struct {
	Name string `json:"memberName"`
	Days []day  `json:"days"`
}

type day struct {
	Name   string  `json:"dayName"`
	Events []event `json:"events"`
}

type event struct {
	Id   string `json:"eventId"`
	Name string `json:"eventName"`
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	json.NewEncoder(f).Encode(token)
}

func loadCalendars(path string) []cal {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	var c cals
	json.Unmarshal(raw, &c)

	return c.Calendars
}

var calendars []cal
var dayKeys map[string]int
var srv *calendar.Service
var family familyCalendar

var days = [...]string{
	"Sunday",
	"Monday",
	"Tuesday",
	"Wednesday",
	"Thursday",
	"Friday",
	"Saturday",
}

func init() {
	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved client_secret.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	srv, err = calendar.New(getClient(config))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	calendars = loadCalendars("./calendars.json")

	family.MemberCalendars = make([]memberCalendar, len(calendars))
}

func main() {
	var wg sync.WaitGroup

	updateLocalCalendarEvents()
	ticker := time.NewTicker(30 * time.Second)
	quit := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ticker.C:
				updateLocalCalendarEvents()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		http.HandleFunc("/json", handler)
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	fmt.Println("Running")
	wg.Wait()
	fmt.Println("Closed")

}

func updateLocalCalendarEvents() {
	// Get week Boundaries
	t0, t1 := weekBoundaries(time.Now())

	dayKeys = make(map[string]int, 7)
	//days = make([]string, 7)
	for i := 0; i < 7; i++ {
		dayLabel := t0.Add(time.Hour * time.Duration(24*i)).Format("2006-01-02")
		// fmt.Println(dayLabel)
		dayKeys[dayLabel] = i
		//days[i] = dayLabel
	}

	for i, cal := range calendars {
		events, _ := srv.Events.List(cal.ID).ShowDeleted(false).
			SingleEvents(true).TimeMin(t0.Format(time.RFC3339)).TimeMax(t1.Format(time.RFC3339)).OrderBy("startTime").Do()

		var member memberCalendar
		member.Name = cal.Name

		for _, d := range days {
			var day day
			day.Name = d
			member.Days = append(member.Days, day)
		}

		for _, item := range events.Items {
			itemDayKey := extractDayKey(item)
			bucketIndex := dayKeys[itemDayKey]
			bucket := &member.Days[bucketIndex]

			var event event
			event.Id = item.Id
			event.Name = item.Summary

			bucket.Events = append(bucket.Events, event)
		}

		family.MemberCalendars[i] = member
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(family)
}

func extractDayKey(item *calendar.Event) string {
	if item.OriginalStartTime != nil {
		if item.OriginalStartTime.Date != "" {
			return item.OriginalStartTime.Date
		}
		if item.OriginalStartTime.DateTime != "" {
			return item.OriginalStartTime.DateTime[0:10]
		}
	}
	if item.Start.Date != "" {
		return item.Start.Date
	}

	if item.Start.DateTime != "" {
		return item.Start.DateTime[0:10]
	}

	return ""
}

func weekBoundaries(t time.Time) (beginning time.Time, end time.Time) {
	//daysSinceMonday := int(t.Weekday()+6) % 7 * -1

	fmt.Println(t)
	fmt.Println(t.Weekday())
	daysSinceSunday := int(t.Weekday())
	fmt.Println(daysSinceSunday)
	offsetFromWeekBeginning := time.Hour * time.Duration(-24*daysSinceSunday)
	beginning = t.Truncate(time.Hour * 24).Add(offsetFromWeekBeginning)
	end = beginning.Add(time.Hour * (24 * 7))
	return
}
