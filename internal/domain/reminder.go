package domain

// Reminder indica con cuanta anticipacion se notifica al usuario.
type Reminder string

const (
	ReminderNone         Reminder = "none"
	ReminderAtTime       Reminder = "at_time"
	ReminderTenMinutes   Reminder = "ten_minutes_before"
	ReminderOneDayBefore Reminder = "one_day_before"
)

var validReminders = map[Reminder]bool{
	ReminderNone:         true,
	ReminderAtTime:       true,
	ReminderTenMinutes:   true,
	ReminderOneDayBefore: true,
}

// ParseReminder valida y normaliza un recordatorio.
func ParseReminder(s string) (Reminder, error) {
	r := Reminder(s)
	if !validReminders[r] {
		return "", ErrInvalidReminder
	}
	return r, nil
}

// Valid indica si el recordatorio es uno de los permitidos.
func (r Reminder) Valid() bool {
	return validReminders[r]
}
