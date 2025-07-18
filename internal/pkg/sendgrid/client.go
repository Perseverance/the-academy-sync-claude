// Package sendgrid provides a client wrapper for SendGrid email service
package sendgrid

import (
	"fmt"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// Client wraps the SendGrid client with additional configuration
type Client struct {
	client    *sendgrid.Client
	fromEmail string
	fromName  string
}

// NewClient creates a new SendGrid client wrapper
func NewClient(apiKey, fromEmail string) *Client {
	return &Client{
		client:    sendgrid.NewSendClient(apiKey),
		fromEmail: fromEmail,
		fromName:  "The Academy Sync",
	}
}

// SendEmail sends an email using SendGrid
func (c *Client) SendEmail(toEmail, toName, subject, plainTextContent, htmlContent string) error {
	from := mail.NewEmail(c.fromName, c.fromEmail)
	to := mail.NewEmail(toName, toEmail)
	
	message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)
	
	response, err := c.client.Send(message)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	
	// Check if the response indicates an error
	if response.StatusCode >= 400 {
		return fmt.Errorf("sendgrid returned error status %d: %s", response.StatusCode, response.Body)
	}
	
	return nil
}

// SendTemplateEmail sends an email using a SendGrid dynamic template
func (c *Client) SendTemplateEmail(toEmail, toName, templateID string, templateData map[string]interface{}) error {
	from := mail.NewEmail(c.fromName, c.fromEmail)
	to := mail.NewEmail(toName, toEmail)
	
	message := mail.NewV3Mail()
	message.SetFrom(from)
	message.SetTemplateID(templateID)
	
	p := mail.NewPersonalization()
	p.AddTos(to)
	
	// Add dynamic template data
	for key, value := range templateData {
		p.SetDynamicTemplateData(key, value)
	}
	
	message.AddPersonalizations(p)
	
	response, err := c.client.Send(message)
	if err != nil {
		return fmt.Errorf("failed to send template email: %w", err)
	}
	
	// Check if the response indicates an error
	if response.StatusCode >= 400 {
		return fmt.Errorf("sendgrid returned error status %d: %s", response.StatusCode, response.Body)
	}
	
	return nil
}