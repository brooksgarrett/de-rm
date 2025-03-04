package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"socialbot/config"
	"socialbot/tools"

	"github.com/golang/glog"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type SocialAssistant struct {
	model *genai.Client
	ctx   context.Context
}

func NewSocialAssistant() (*SocialAssistant, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	return &SocialAssistant{
		model: client,
		ctx:   ctx,
	}, nil
}

func (s *SocialAssistant) Chat(input string) (string, error) {
	model := s.model.GenerativeModel("gemini-pro")
	tokResp, err := model.CountTokens(s.ctx, genai.Text(input))
	if err != nil {
		glog.Errorf("Error counting tokens: %v", err)
	}

	// Add debug logging for the prompt
	glog.Infof("Sending prompt to Gemini:\n%s", input)

	glog.Infof("Chatting with Gemini: %d tokens", tokResp.TotalTokens)
	resp, err := model.GenerateContent(s.ctx, genai.Text(input))
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %v", err)
	}

	return fmt.Sprint(resp.Candidates[0].Content.Parts[0]), nil
}

func (s *SocialAssistant) GetSocialRecommendations() (string, error) {
	calendarTool := tools.NewCalendarTool()
	emailTool := tools.NewEmailTool()

	// Get data from last 30 days
	since := time.Now().AddDate(0, 0, -30)

	events, err := calendarTool.GetRecentEvents(s.ctx, since)
	if err != nil {
		return "", fmt.Errorf("failed to get calendar events: %v", err)
	}

	interactions, err := emailTool.GetRecentInteractions(s.ctx, since, "")
	if err != nil {
		return "", fmt.Errorf("failed to get email interactions: %v", err)
	}

	// Format data for Gemini
	prompt := formatSocialDataPrompt(events, interactions)
	return s.Chat(prompt)
}

func (s *SocialAssistant) DraftEmail(to string) (string, error) {
	emailTool := tools.NewEmailTool()
	rssReader := tools.NewRSSReader()

	// Find the specific contact and their details
	var targetInteraction *tools.EmailInteraction
	var targetContact *config.Contact
	contacts := config.GetImportantContacts()

	for _, contact := range contacts {
		if contact.Email == to {
			targetContact = &contact
			break
		}
	}

	if targetContact == nil {
		return "", fmt.Errorf("contact not found in important contacts: %s", to)
	}

	interactions, err := emailTool.GetInteractionsByParticipant(s.ctx, targetContact.Email)
	if err != nil {
		return "", fmt.Errorf("failed to get email interactions: %v", err)
	}

	for _, interaction := range interactions {
		if interaction.Participant == to {
			targetInteraction = &interaction
			break
		}
	}

	// Get their recent blog posts if available
	var recentPosts []tools.BlogPost
	if targetContact.RSSFeed != "" {
		posts, err := rssReader.GetRecentPosts(targetContact.RSSFeed, 3)
		if err != nil {
			glog.Warningf("Warning: Failed to fetch RSS feed: %v", err)
		} else {
			recentPosts = posts
		}
	}

	prompt := formatEmailDraftPrompt(targetContact, targetInteraction, recentPosts)
	response, err := s.Chat(prompt)
	if err != nil {
		return "", err
	}

	// Parse the response into subject and body
	draft, err := parseEmailResponse(response)
	if err != nil {
		return "", fmt.Errorf("failed to parse email response: %v", err)
	}

	// Create draft in Gmail
	draft.To = to
	if err := emailTool.SaveDraft(s.ctx, draft); err != nil {
		return "", fmt.Errorf("failed to save draft: %v", err)
	}

	return fmt.Sprintf("Draft saved to Gmail:\n\n%s", response), nil
}

func parseEmailResponse(response string) (tools.DraftEmail, error) {
	lines := strings.Split(response, "\n")
	var draft tools.DraftEmail
	var bodyLines []string
	inBody := false

	for _, line := range lines {
		if strings.HasPrefix(line, "Subject: ") {
			draft.Subject = strings.TrimPrefix(line, "Subject: ")
		} else if line == "" && draft.Subject != "" {
			inBody = true
		} else if inBody {
			bodyLines = append(bodyLines, line)
		}
	}

	if draft.Subject == "" {
		return draft, fmt.Errorf("no subject found in response")
	}

	draft.Body = strings.Join(bodyLines, "\n")
	return draft, nil
}

func formatEmailDraftPrompt(contact *config.Contact, interaction *tools.EmailInteraction, posts []tools.BlogPost) string {
	var context strings.Builder

	if interaction != nil {
		fmt.Fprintf(&context, "Last contact was on %s, with %d total interactions. ",
			interaction.LastContact.Format("2006-01-02"),
			interaction.Count)
	} else {
		context.WriteString("No previous email interactions found. ")
	}

	if len(posts) > 0 {
		context.WriteString("\n\nRecent blog posts:\n")
		for _, post := range posts {
			fmt.Fprintf(&context, "- %s (published %s)\n  %s\n",
				post.Title,
				post.Published.Format("2006-01-02"),
				post.Link)
		}
	}

	writingSample := "No writing sample available."
	if contact.WritingSample != "" {
		writingSample = contact.WritingSample
	}

	return fmt.Sprintf(`Draft a friendly email to %s (%s).

Here's an example of how I write emails:
---
%s
---

Context about our relationship: %s

Please write a natural, personal email that:
1. Has an appropriate subject line
2. Matches my writing style and tone from the example
3. Includes a specific reference to our last interaction if available
4. If they have recent blog posts, mention one that interested you
5. Ends with a clear next step or question
6. Uses similar greeting/closing styles as my example

Format the response as:
Subject: [subject]

[email body]`, contact.Name, contact.Email, writingSample, context.String())
}

func formatSocialDataPrompt(events []tools.Event, interactions []tools.EmailInteraction) string {
	return fmt.Sprintf(`Based on the following data about my important contacts, who should I reach out to this week?

Calendar Events (Last 30 days):
%v

Important Contact Interactions (Last 30 days):
%v

Please recommend 3 or less important contacts I should reach out to this week. 
Consider factors like:
1. Contact priority (1-5, where 5 is highest)
2. Time since last contact
3. Frequency of past interactions
4. Any upcoming events
`, formatEvents(events), formatInteractions(interactions))
}

func formatEvents(events []tools.Event) string {
	var result strings.Builder
	for _, event := range events {
		result.WriteString(fmt.Sprintf("- %s with %s on %s\n",
			event.Title,
			strings.Join(event.Attendees, ", "),
			event.StartTime.Format("2006-01-02")))
	}
	return result.String()
}

func formatInteractions(interactions []tools.EmailInteraction) string {
	var result strings.Builder
	for _, interaction := range interactions {
		result.WriteString(fmt.Sprintf("- %s (%s) [Priority: %d] (Last contact: %s, Total interactions: %d)\n",
			interaction.Name,
			interaction.Participant,
			interaction.Priority,
			interaction.LastContact.Format("2006-01-02"),
			interaction.Count))
	}
	return result.String()
}

func (s *SocialAssistant) CatchupWithBlog(email string) (string, error) {
	// Find the contact
	contacts := config.GetImportantContacts()
	var targetContact *config.Contact
	for _, contact := range contacts {
		if contact.Email == email {
			targetContact = &contact
			break
		}
	}

	if targetContact == nil {
		return "", fmt.Errorf("contact not found in important contacts: %s", email)
	}

	if targetContact.RSSFeed == "" {
		return "", fmt.Errorf("no RSS feed configured for %s", targetContact.Name)
	}

	// Get recent posts
	rssReader := tools.NewRSSReader()
	posts, err := rssReader.GetRecentPosts(targetContact.RSSFeed, 10) // Get more posts to filter by date
	if err != nil {
		return "", fmt.Errorf("failed to fetch RSS feed: %v", err)
	}

	// Filter to last week
	weekAgo := time.Now().AddDate(0, 0, -30)
	var recentPosts []tools.BlogPost
	for _, post := range posts {
		if post.Published.After(weekAgo) {
			recentPosts = append(recentPosts, post)
		}
	}

	if len(recentPosts) == 0 {
		return fmt.Sprintf("No posts from %s in the last week.", targetContact.Name), nil
	}

	prompt := formatCatchupPrompt(targetContact, recentPosts)
	return s.Chat(prompt)
}

func formatCatchupPrompt(contact *config.Contact, posts []tools.BlogPost) string {
	var postsBuilder strings.Builder
	for _, post := range posts {
		fmt.Fprintf(&postsBuilder, "- %s (published %s)\n  %s\n\n",
			post.Title,
			post.Published.Format("2006-01-02"),
			post.Link)
	}

	return fmt.Sprintf(`Summarize these recent blog posts from %s:

%s
Please provide:
1. A brief overview of the main themes/topics covered
2. Key insights or interesting points from each post
3. Any actionable takeaways
4. Potential discussion points I could bring up in a conversation with the author

Keep the summary concise but informative.`, contact.Name, postsBuilder.String())
}

func main() {
	cmd := flag.String("cmd", "recommend", "Command to run: 'recommend', 'draft', or 'catchup'")
	email := flag.String("email", "", "Email address for draft/catchup command")
	flag.Parse()

	assistant, err := NewSocialAssistant()
	if err != nil {
		glog.Exitf("Failed to initialize assistant: %v", err)
	}
	defer assistant.model.Close()

	switch *cmd {
	case "recommend":
		glog.Infof("Getting social recommendations")
		recommendations, err := assistant.GetSocialRecommendations()
		if err != nil {
			glog.Exitf("Failed to get recommendations: %v", err)
		}
		fmt.Println("Social Recommendations:")
		fmt.Println(recommendations)

	case "draft":
		glog.Infof("Drafting email to %s", *email)
		if *email == "" {
			glog.Fatal("Email address is required for draft command")
		}
		draft, err := assistant.DraftEmail(*email)
		if err != nil {
			glog.Exitf("Failed to draft email: %v", err)
		}
		fmt.Println("Email Draft:")
		fmt.Println(draft)

	case "catchup":
		glog.Infof("Getting blog catchup for %s", *email)
		if *email == "" {
			glog.Fatal("Email address is required for catchup command")
		}
		summary, err := assistant.CatchupWithBlog(*email)
		if err != nil {
			glog.Exitf("Failed to get blog catchup: %v", err)
		}
		fmt.Println("Blog Catchup Summary:")
		fmt.Println(summary)

	default:
		glog.Exitf("Unknown command: %s", *cmd)
	}
}
