# Family Dashboard

A replacement for the whiteboard calendar that we maintain.  Each family member gets a row, and 7 columns allow for a week of activities to be displayed.

Whiteboard was difficult to maintain, and as most of us were also keeping a google calendar as well, automation seemed like a winner.

NB: This is a first pass, and there is substantial room for improvement

- automatic updating of events
- alternate between this week, and the following
- initial load waits for 30 seconds before populating the calendar entries

## Initial Setup

### Turn on the API in your google account
1. Use this [wizard](https://console.developers.google.com/start/api?id=calendar) to enable the API
1. Select "Create New Project" and click continue
1. Click "Go to Credentials"
1. Click on the "Cancel" button
1. Select the "OAuth consent screen" tab
1. Add an Email address, enter a Product name if not already set, and click the Save button.
1. Select the Credentials tab, click the Create credentials button and select OAuth client ID
1. Select the application type Other, enter the name "Google Calendar API Quickstart", and click the Create button.
1. Click OK to dismiss the resulting dialog.
1. Download the JSON object.
1. Rename it to client_secret.json and copy it to the src folder

### Choose which calendars you want to display
Create a calendars.json file, and populate it with the list of Calendars to be displayed.  The name is used only in display.  The id can be retrieved from the calendars settings.

NB The user that authorises the application must have access to each calendar that is intended to be displayed.

```:json
{
    "calendars": [
        {
            "name": "Cal1",
            "id": "calendarId"
        },
        {
            "name": "Cal2",
            "id": "calendar2Id"
        }
    ]
}
```

### Build and initial run the application

`go install`
family-dashboard

Browse to the URL that is outputted.
Enter the Authorization Code at the console

A `token.json` file will then be created, allowing the app to use those credentials.

### Browse to the dashboard

Browse to the [dashboard](http://localhost:8080/)