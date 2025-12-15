package services

type EmailSender interface {
	Send(to string, subject string, body string) error
}
