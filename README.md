# Social Relationship Manager

A Gemini-powered chatbot that helps you maintain and improve your interpersonal relationships by analyzing your calendar, email history, and contact information to provide personalized recommendations and assistance.

## Features

- **Smart Contact Recommendations**: Analyzes your calendar events and email interactions to suggest who you should reach out to this week
- **Email Drafting**: Automatically drafts personalized emails based on your writing style and the recipient's recent activities
- **Blog Catchup**: Summarizes recent blog posts from your contacts to help you stay informed and engaged
- **Priority-Based Contact Management**: Uses a priority system (1-5) to help you focus on the most important relationships

## Prerequisites

- Go 1.21 or later
- Google Cloud Platform account with Gemini API access
- Gmail API access
- Google Calendar API access

## Configuration

1. Create a `config/contacts.json` file based on the example:
   ```bash
   cp config/contacts_example.json config/contacts.json
   ```

2. Set up your environment variables:
   ```bash
   cp .env.example .env
   ```
   Required environment variables:
   - `GEMINI_API_KEY`: Your Google Gemini API key
   - `GMAIL_CREDENTIALS`: Path to your Gmail API credentials
   - `CALENDAR_CREDENTIALS`: Path to your Google Calendar API credentials

## Usage

The application provides a few main commands:

### Get Social Recommendations
```bash
go run main.go -cmd recommend
```
This will analyze your recent interactions and suggest who you should reach out to this week.

### Draft an Email
```bash
go run main.go -cmd draft -email example@example.com
```
This will draft a personalized email to the specified contact, incorporating their recent activities and your writing style.

### Catch Up on Blog Posts
```bash
go run main.go -cmd catchup -email example@example.com
```
This will provide a summary of the contact's recent blog posts and suggest discussion points.

## Contact Configuration

Each contact in `contacts.json` can have the following fields:
- `email`: Contact's email address
- `name`: Contact's name
- `priority`: Priority level (1-5, where 5 is highest)
- `rss_feed`: URL to their blog's RSS feed (optional)
- `writing_sample`: Example of your writing style for this contact (optional)

## Development

The application is built with:
- Google's Gemini AI for natural language processing
- Gmail API for email management
- Google Calendar API for event tracking
- RSS feed parsing for blog updates

## License

MIT License - see LICENSE file for details 