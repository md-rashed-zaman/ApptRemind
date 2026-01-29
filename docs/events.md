# Event Catalog (Contracts)

This catalog is the contract for event names and payload shape.
All events include headers:
- `event_id` (UUID)
- `event_type` (string, same as topic)

## Auth
- event: auth.user.created.v1
  - producer: auth-service
  - payload:
    - user_id (UUID)
    - business_id (UUID)
    - email (string)
    - created_at (RFC3339)

- event: auth.audit.v1
  - producer: auth-service
  - payload:
    - event_type (string)
    - actor_id (UUID, nullable)
    - metadata (object)
    - created_at (RFC3339)

## Booking
- event: booking.appointment.booked.v1
  - producer: booking-service
  - payload:
    - appointment_id (UUID)
    - business_id (UUID)
    - staff_id (UUID)
    - service_id (UUID)
    - customer_email (string, optional)
    - customer_phone (string, optional)
    - start_time (RFC3339)
    - end_time (RFC3339)

- event: booking.appointment.cancelled.v1
  - producer: booking-service
  - payload:
    - appointment_id (UUID)
    - business_id (UUID)
    - staff_id (UUID)
    - service_id (UUID)
    - start_time (RFC3339)
    - end_time (RFC3339)
    - cancelled_at (RFC3339)
    - reason (string, optional)

- event: booking.reminder.requested.v1
  - producer: booking-service
  - payload:
    - appointment_id (UUID)
    - business_id (UUID)
    - channel (email|sms)
    - recipient (string)
    - remind_at (RFC3339)
    - template_data (object)

## Scheduler
- event: scheduler.reminder.due.v1
  - producer: scheduler-service
  - payload:
    - appointment_id (UUID)
    - business_id (UUID)
    - channel (email|sms)
    - recipient (string)
    - remind_at (RFC3339)
    - template_data (object)

- event: scheduler.reminder.dlq.v1
  - producer: scheduler-service
  - payload:
    - appointment_id (UUID)
    - business_id (UUID)
    - channel (email|sms)
    - recipient (string)
    - remind_at (RFC3339)
    - error_reason (string)
    - failed_at (RFC3339)

## Notification
- event: notification.sent.v1
  - producer: notification-service
  - payload:
    - appointment_id (UUID)
    - business_id (UUID)
    - channel (email|sms)
    - provider_id (string)
    - sent_at (RFC3339)

- event: notification.failed.v1
  - producer: notification-service
  - payload:
    - appointment_id (UUID)
    - business_id (UUID)
    - channel (email|sms)
    - error_reason (string)
    - failed_at (RFC3339)

## Billing
- event: billing.subscription.activated.v1
  - producer: billing-service
  - payload:
    - business_id (UUID)
    - tier (string)
    - max_monthly_appointments (int)
    - activated_at (RFC3339)

- event: billing.subscription.canceled.v1
  - producer: billing-service
  - payload:
    - business_id (UUID)
    - tier (string) = free
    - max_monthly_appointments (int)
    - canceled_at (RFC3339)
