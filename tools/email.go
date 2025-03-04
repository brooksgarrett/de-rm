package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"socialbot/auth"
	"socialbot/config"

	"github.com/golang/glog"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type EmailTool struct {
	service *gmail.Service
}

type EmailInteraction struct {
	Participant string
	Name        string
	Priority    int
	LastContact time.Time
	Count       int
}

type DraftEmail struct {
	Subject string
	Body    string
	To      string
}

func NewEmailTool() *EmailTool {
	ctx := context.Background()
	client, err := auth.GetClient()
	if err != nil {
		glog.Exitf("Unable to get OAuth client: %v", err)
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		glog.Exitf("Unable to create Gmail service: %v", err)
	}

	return &EmailTool{
		service: srv,
	}
}

func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser: \n%v\n", authURL)

	var authCode string
	fmt.Print("Enter the authorization code: ")
	if _, err := fmt.Scan(&authCode); err != nil {
		glog.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		glog.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		glog.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (e *EmailTool) filterAndEnrichInteractions(interactions []EmailInteraction) []EmailInteraction {
	importantContacts := config.GetImportantContacts()
	contactMap := make(map[string]config.Contact)
	for _, contact := range importantContacts {
		contactMap[contact.Email] = contact
	}

	var filtered []EmailInteraction
	for _, interaction := range interactions {
		if contact, ok := contactMap[interaction.Participant]; ok {
			interaction.Name = contact.Name
			interaction.Priority = contact.Priority
			filtered = append(filtered, interaction)
		}
	}
	return filtered
}

func (e *EmailTool) GetInteractionsByParticipant(ctx context.Context, participant string) ([]EmailInteraction, error) {
	query := fmt.Sprintf("(from:%s OR to:%s)", participant, participant)
	return e.GetRecentInteractions(ctx, time.Now().AddDate(0, 0, -30), query)
}

func (e *EmailTool) GetRecentInteractions(ctx context.Context, since time.Time, query string) ([]EmailInteraction, error) {
	// Format the date as YYYY/MM/DD for Gmail's query syntax
	query += fmt.Sprintf(" after:%s", since.Format("2006/01/02"))
	glog.Infof("Querying emails with: %s", query)

	messages, err := e.service.Users.Messages.
		List("me").
		Q(query).
		MaxResults(500). // Add reasonable limit
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %v", err)
	}

	glog.Infof("Found %d total messages", len(messages.Messages))

	interactions := make(map[string]*EmailInteraction)

	for _, msg := range messages.Messages {
		message, err := e.service.Users.Messages.Get("me", msg.Id).Do()
		if err != nil {
			glog.V(2).Infof("Error getting message %s: %v", msg.Id, err)
			continue
		}

		var from string
		var date time.Time

		for _, header := range message.Payload.Headers {
			switch header.Name {
			case "From":
				from = extractEmail(header.Value)
				glog.V(2).Infof("Found email from: %s (raw: %s)", from, header.Value)
			case "Date":
				date, _ = parseEmailDate(header.Value)
			}
		}

		if from == "" {
			glog.V(2).Infof("Skipping message %s: no From header", msg.Id)
			continue
		}

		if interaction, exists := interactions[from]; exists {
			interaction.Count++
			if date.After(interaction.LastContact) {
				interaction.LastContact = date
			}
		} else {
			interactions[from] = &EmailInteraction{
				Participant: from,
				LastContact: date,
				Count:       1,
			}
		}
	}

	result := make([]EmailInteraction, 0, len(interactions))
	for _, interaction := range interactions {
		result = append(result, *interaction)
	}

	filtered := e.filterAndEnrichInteractions(result)
	glog.Infof("Found %d total interactions, filtered to %d important contacts", len(result), len(filtered))

	return filtered, nil
}

func extractEmail(from string) string {
	// Handle different email formats
	// Format 1: "Name <email@domain.com>"
	if start := strings.Index(from, "<"); start >= 0 {
		if end := strings.Index(from[start:], ">"); end >= 0 {
			return strings.TrimSpace(from[start+1 : start+end])
		}
	}

	// Format 2: "email@domain.com (Name)"
	if end := strings.Index(from, " ("); end >= 0 {
		return strings.TrimSpace(from[:end])
	}

	// Format 3: Just "email@domain.com"
	return strings.TrimSpace(from)
}

func parseEmailDate(date string) (time.Time, error) {
	return time.Parse(time.RFC1123Z, date)
}

func (e *EmailTool) SaveDraft(ctx context.Context, draft DraftEmail) error {
	var message bytes.Buffer

	// Format email according to RFC 822
	fmt.Fprintf(&message, "From: me\n")
	fmt.Fprintf(&message, "To: %s\n", draft.To)
	fmt.Fprintf(&message, "Subject: %s\n", draft.Subject)
	fmt.Fprintf(&message, "Content-Type: text/plain; charset=UTF-8\n")
	fmt.Fprintf(&message, "\n%s", draft.Body)

	// Create the draft
	gmailDraft := &gmail.Draft{
		Message: &gmail.Message{
			Raw: base64.URLEncoding.EncodeToString(message.Bytes()),
		},
	}

	_, err := e.service.Users.Drafts.Create("me", gmailDraft).Do()
	if err != nil {
		return fmt.Errorf("failed to create draft: %v", err)
	}

	glog.Infof("Draft saved for %s", draft.To)
	return nil
}
