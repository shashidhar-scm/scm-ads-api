package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"scm/internal/interfaces"
	"scm/internal/models"
	"scm/internal/repository"
)

type CampaignNotifier struct {
	repo        interfaces.CampaignRepository
	userRepo    repository.UserRepository
	emailSender EmailSender
}

func NewCampaignNotifier(repo interfaces.CampaignRepository, userRepo repository.UserRepository, emailSender EmailSender) *CampaignNotifier {
	return &CampaignNotifier{
		repo:        repo,
		userRepo:    userRepo,
		emailSender: emailSender,
	}
}

func (n *CampaignNotifier) SendActivationNotifications(ctx context.Context) error {
	now := time.Now().UTC()
	oneDayLater := now.AddDate(0, 0, 1)

	// Find campaigns that start today (become active)
	activeCampaigns, err := n.repo.ListByStartDate(ctx, now.Truncate(24*time.Hour))
	if err != nil {
		return fmt.Errorf("failed to list campaigns starting today: %w", err)
	}

	// Find campaigns that start tomorrow (reminder)
	reminderCampaigns, err := n.repo.ListByStartDate(ctx, oneDayLater.Truncate(24*time.Hour))
	if err != nil {
		return fmt.Errorf("failed to list campaigns starting tomorrow: %w", err)
	}

	// Send activation emails
	for _, campaign := range activeCampaigns {
		if campaign.Status != "active" {
			continue // only send if becoming active
		}
		if err := n.sendActivationEmail(ctx, campaign); err != nil {
			log.Printf("Failed to send activation email for campaign %s: %v", campaign.ID, err)
		}
	}

	// Send reminder emails
	for _, campaign := range reminderCampaigns {
		if err := n.sendReminderEmail(ctx, campaign); err != nil {
			log.Printf("Failed to send reminder email for campaign %s: %v", campaign.ID, err)
		}
	}

	return nil
}

func (n *CampaignNotifier) SendCompletionNotifications(ctx context.Context, endDate time.Time) error {
	endedCampaigns, err := n.repo.ListByEndDate(ctx, endDate.Truncate(24*time.Hour))
	if err != nil {
		return fmt.Errorf("failed to list campaigns ending today: %w", err)
	}

	for _, campaign := range endedCampaigns {
		if err := n.sendCompletionEmail(ctx, campaign); err != nil {
			log.Printf("Failed to send completion email for campaign %s: %v", campaign.ID, err)
		}
	}

	return nil
}

func (n *CampaignNotifier) sendActivationEmail(ctx context.Context, campaign *models.Campaign) error {
	user, err := n.userRepo.GetByID(ctx, campaign.AdvertiserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	subject := fmt.Sprintf("Your campaign '%s' is now live!", campaign.Name)
	body := fmt.Sprintf(`
Hello %s,

Your campaign '%s' has become active and is now live.

Campaign Details:
- Name: %s
- Start Date: %s
- End Date: %s

You can view the campaign at: https://scm-ads-api.citypost.us/api/v1/campaigns/%s

Best regards,
SCM Ads Team
`, user.Name, campaign.Name, campaign.Name, campaign.StartDate.Format("2006-01-02"), campaign.EndDate.Format("2006-01-02"), campaign.ID)

	return n.emailSender.Send(user.Email, subject, body)
}

func (n *CampaignNotifier) sendCompletionEmail(ctx context.Context, campaign *models.Campaign) error {
	user, err := n.userRepo.GetByID(ctx, campaign.AdvertiserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	subject := fmt.Sprintf("Your campaign '%s' has completed", campaign.Name)
	body := fmt.Sprintf(`
Hello %s,

Your campaign '%s' ended on %s. Thank you for running it with Smart City Media.

Campaign Details:
- Name: %s
- Start Date: %s
- End Date: %s

You can review campaign performance at: https://scm-ads.citypost.us/campaigns/%s

Best regards,
SCM Ads Team
`, user.Name, campaign.Name, campaign.EndDate.Format("2006-01-02"), campaign.Name, campaign.StartDate.Format("2006-01-02"), campaign.EndDate.Format("2006-01-02"), campaign.ID)

	return n.emailSender.Send(user.Email, subject, body)
}

func (n *CampaignNotifier) sendReminderEmail(ctx context.Context, campaign *models.Campaign) error {
	user, err := n.userRepo.GetByID(ctx, campaign.AdvertiserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	subject := fmt.Sprintf("Reminder: Your campaign '%s' goes live tomorrow!", campaign.Name)
	body := fmt.Sprintf(`
Hello %s,

This is a reminder that your campaign '%s' is scheduled to go live tomorrow.

Campaign Details:
- Name: %s
- Start Date: %s
- End Date: %s

You can view the campaign at: https://scm-ads-api.citypost.us/api/v1/campaigns/%s

Best regards,
SCM Ads Team
`, user.Name, campaign.Name, campaign.Name, campaign.StartDate.Format("2006-01-02"), campaign.EndDate.Format("2006-01-02"), campaign.ID)

	return n.emailSender.Send(user.Email, subject, body)
}
