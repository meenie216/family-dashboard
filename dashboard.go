package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

type cals struct {
	Calendars []cal `jsob:"calendars"`
}

type cal struct {
	Name string `json:"name"`
	ID   string `json:"id"`
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

var eventsByDayByCalendar [][][]*calendar.Event
var calendars []cal
var dayKeys map[string]int
var srv *calendar.Service

//var days []string

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
}

func main() {

	ticker := time.NewTicker(30 * time.Second)
	quit := make(chan struct{})
	go func() {
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

	http.HandleFunc("/", handler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	log.Fatal(http.ListenAndServe(":8080", nil))

	for cal, eventsByDay := range eventsByDayByCalendar {
		fmt.Println(calendars[cal].Name)
		for day, events := range eventsByDay {
			fmt.Println(day)
			for i := range events {
				item := events[i]
				fmt.Println(item.Summary)
				itemDayKey := item.Start.Date
				if item.RecurringEventId != "" {
					itemDayKey = item.OriginalStartTime.Date
				}
				fmt.Println(itemDayKey)
			}
		}
	}
}

func updateLocalCalendarEvents() {
	// initialise slices
	eventsByDayByCalendar = make([][][]*calendar.Event, len(calendars))
	for calendarIndex := range eventsByDayByCalendar {
		eventsByDayByCalendar[calendarIndex] = make([][]*calendar.Event, 7)
		for dayIndex := range eventsByDayByCalendar[calendarIndex] {
			eventsByDayByCalendar[calendarIndex][dayIndex] = make([]*calendar.Event, 0)
		}
	}

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

		for _, item := range events.Items {

			itemDayKey := extractDayKey(item)
			bucketIndex := dayKeys[itemDayKey]

			eventsByDayByCalendar[i][bucketIndex] = append(eventsByDayByCalendar[i][bucketIndex], item)
		}
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "<html><head>")
	fmt.Fprint(w, "<link rel='stylesheet' type='text/css' href='static/dashboard.css'>")
	fmt.Fprint(w, "<link href='https://fonts.googleapis.com/css?family=Indie+Flower|Architects+Daughter|Boogaloo|Caveat+Brush|Chewy|Dokdo|Gochi+Hand' rel='stylesheet'>")
	fmt.Fprint(w, "</head><body>")

	fmt.Fprint(w, "<ul class='names'>")
	// first print the day headers
	fmt.Fprint(w, "<li class='name'><ul class='days'>")
	for i := 0; i < 7; i++ {
		fmt.Fprintf(w, "<li class='day dayHeader'>%s", days[i])
	}
	fmt.Fprint(w, "</ul>")

	// Next iterate over collection and print all events retrieved
	for cal, eventsByDay := range eventsByDayByCalendar {
		fmt.Fprintf(w, "<li class='name'><p class='calendar-name'>%s</p>", calendars[cal].Name)
		fmt.Fprint(w, "<ul class='days'>")
		for day, events := range eventsByDay {
			fmt.Fprintf(w, "<li class='day'><p class='daylabel'>%s</p><ul class='events'>", days[day])
			for i := range events {
				item := events[i]
				fmt.Fprintf(w, "<li class='event'>%s", item.Summary)
			}
			fmt.Fprint(w, "</ul>")
		}
		fmt.Fprint(w, "</ul>")
	}
	fmt.Fprint(w, "</ul></body></html>")
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
	daysSinceMonday := int(t.Weekday())
	offsetFromWeekBeginning := time.Hour * time.Duration(-24*daysSinceMonday)
	beginning = t.Round(time.Hour * 24).Add(offsetFromWeekBeginning)
	end = beginning.Add(time.Hour * (24 * 7))
	return
}
