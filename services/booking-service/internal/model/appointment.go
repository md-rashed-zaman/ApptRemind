package model

import "time"

type Appointment struct {
	ID            string
	BusinessID    string
	ServiceID     string
	StaffID       string
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	StartTime     time.Time
	EndTime       time.Time
	Status        string
	CancelledAt   *time.Time
	CancelReason  string
	CreatedAt     time.Time
}
