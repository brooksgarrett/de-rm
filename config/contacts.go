package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
)

type Contact struct {
	Email         string
	Name          string
	Priority      int
	RSSFeed       string `json:"rss_feed,omitempty"`
	WritingSample string `json:"writing_sample,omitempty"`
}

func (c *Contact) validate() error {
	if strings.TrimSpace(c.Email) == "" {
		return fmt.Errorf("email is required")
	}
	if !strings.Contains(c.Email, "@") {
		return fmt.Errorf("invalid email format: %s", c.Email)
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("name is required for %s", c.Email)
	}
	if c.Priority < 1 || c.Priority > 5 {
		return fmt.Errorf("priority must be between 1-5 for %s", c.Email)
	}
	return nil
}

func GetImportantContacts() []Contact {
	file, err := os.ReadFile("config/contacts.json")
	if err != nil {
		glog.Exitf("Failed to read contacts file: %v", err)
	}

	var contacts []Contact
	if err := json.Unmarshal(file, &contacts); err != nil {
		glog.Exitf("Failed to parse contacts: %v", err)
	}

	for _, contact := range contacts {
		if err := contact.validate(); err != nil {
			glog.Exitf("Invalid contact data: %v", err)
		}
	}

	return contacts
}
