package models

// ReminderTemplateItem is one reminder of a template: what it tracks and how often.
type ReminderTemplateItem struct {
	Title          string `json:"title"`
	Category       string `json:"category"`
	IntervalKm     *int   `json:"interval_km"`
	IntervalMonths *int   `json:"interval_months"`
	LeadKm         int    `json:"lead_km"`
	LeadDays       int    `json:"lead_days"`
}

// ReminderTemplate is a named set of reminders typed by the user; no manufacturer plan is built in.
type ReminderTemplate struct {
	ID     string                 `json:"id"`
	UserID string                 `json:"-"`
	Name   string                 `json:"name"`
	Items  []ReminderTemplateItem `json:"items"`
}
