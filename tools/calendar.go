package tools

import (
	"context"
	"fmt"
	"time"

	"socialbot/auth"

	"github.com/golang/glog"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type CalendarTool struct {
	service *calendar.Service
}

type Event struct {
	Title       string
	StartTime   time.Time
	EndTime     time.Time
	Attendees   []string
	Description string
}

func NewCalendarTool() *CalendarTool {
	ctx := context.Background()
	client, err := auth.GetClient()
	if err != nil {
		glog.Exitf("Unable to get OAuth client: %v", err)
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		glog.Exitf("Unable to create Calendar service: %v", err)
	}

	return &CalendarTool{
		service: srv,
	}
}

func (c *CalendarTool) GetRecentEvents(ctx context.Context, since time.Time) ([]Event, error) {
	events, err := c.service.Events.List("primary").
		TimeMin(since.Format(time.RFC3339)).
		TimeMax(time.Now().Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve events: %v", err)
	}

	glog.Infof("Found %d calendar events since %s", len(events.Items), since.Format("2006-01-02"))

	var result []Event
	for _, item := range events.Items {
		startTime, _ := time.Parse(time.RFC3339, item.Start.DateTime)
		endTime, _ := time.Parse(time.RFC3339, item.End.DateTime)

		attendees := make([]string, 0, len(item.Attendees))
		for _, attendee := range item.Attendees {
			attendees = append(attendees, attendee.Email)
			glog.V(2).Infof("Event '%s' includes attendee: %s", item.Summary, attendee.Email)
		}

		result = append(result, Event{
			Title:       item.Summary,
			StartTime:   startTime,
			EndTime:     endTime,
			Attendees:   attendees,
			Description: item.Description,
		})
	}

	glog.Infof("Processed %d calendar events with attendees", len(result))
	return result, nil
}
