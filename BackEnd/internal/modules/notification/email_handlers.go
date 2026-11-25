package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/smtp"
)

// Canonical outbox event types handled by the notification module.
const (
	EventPasswordResetRequested      = "identity.password_reset_requested"
	EventEmployeeActivationRequested = "identity.employee_activation_requested"
	EventBookingConfirmed            = "booking.confirmed"
	EventBookingCancelled            = "booking.cancelled"
	EventPaymentLateCredited         = "payment.late_transfer_credited"
	EventPaymentRefundCredited       = "payment.refund_credited"
	EventStayCheckedOut              = "stay.checked_out"
)

// PasswordResetEmailPayload is the outbox payload for identity.password_reset_requested.
// EncryptedToken holds the AES-256-GCM encrypted one-time token so plaintext tokens never sit in outbox_events.
type PasswordResetEmailPayload struct {
	UserID         uuid.UUID `json:"userId"`
	Email          string    `json:"email"`
	DisplayName    string    `json:"displayName"`
	EncryptedToken string    `json:"encryptedToken"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

// EmployeeActivationEmailPayload is the outbox payload for identity.employee_activation_requested.
// EncryptedToken holds the AES-256-GCM encrypted one-time activation token.
type EmployeeActivationEmailPayload struct {
	UserID         uuid.UUID `json:"userId"`
	EmployeeID     uuid.UUID `json:"employeeId"`
	EmployeeCode   string    `json:"employeeCode"`
	Email          string    `json:"email"`
	FullName       string    `json:"fullName"`
	PositionTitle  string    `json:"positionTitle"`
	EncryptedToken string    `json:"encryptedToken"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

// GenericEmailPayload is used by booking/payment/stay notification skeletons.
type GenericEmailPayload struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Subject     string `json:"subject"`
	Summary     string `json:"summary"`
}

// EmailDispatcher renders and sends transactional emails from outbox events with in-memory idempotency dedupe.
type EmailDispatcher struct {
	sender          smtp.Sender
	cipher          *auth.PayloadCipher
	frontendBaseURL string
	publicBaseURL   string

	mu   sync.Mutex
	sent map[uuid.UUID]struct{}
}

// EmailDispatcherConfig configures the notification email dispatcher.
type EmailDispatcherConfig struct {
	Sender          smtp.Sender
	Cipher          *auth.PayloadCipher
	FrontendBaseURL string
	PublicBaseURL   string
}

// NewEmailDispatcher creates an EmailDispatcher and registers all outbox email handlers on worker (if non-nil).
func NewEmailDispatcher(cfg EmailDispatcherConfig, worker *Worker) *EmailDispatcher {
	feURL := strings.TrimRight(strings.TrimSpace(cfg.FrontendBaseURL), "/")
	pubURL := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")
	if feURL == "" {
		feURL = pubURL
	}
	if feURL == "" {
		feURL = "http://localhost:5173"
	}
	d := &EmailDispatcher{
		sender:          cfg.Sender,
		cipher:          cfg.Cipher,
		frontendBaseURL: feURL,
		publicBaseURL:   pubURL,
		sent:            make(map[uuid.UUID]struct{}),
	}
	if worker != nil {
		d.RegisterHandlers(worker)
	}
	return d
}

// RegisterHandlers binds all email outbox event types to worker.
func (d *EmailDispatcher) RegisterHandlers(worker *Worker) {
	worker.Register(EventPasswordResetRequested, d.handlePasswordReset)
	worker.Register(EventEmployeeActivationRequested, d.handleEmployeeActivation)
	worker.Register(EventBookingConfirmed, d.handleGenericNotification("Xác nhận đặt phòng thành công"))
	worker.Register(EventBookingCancelled, d.handleGenericNotification("Thông báo hủy đặt phòng"))
	worker.Register(EventPaymentLateCredited, d.handleGenericNotification("Thông báo cộng ví từ giao dịch chuyển khoản trễ"))
	worker.Register(EventPaymentRefundCredited, d.handleGenericNotification("Thông báo hoàn tiền vào ví khách hàng"))
	worker.Register(EventStayCheckedOut, d.handleGenericNotification("Hóa đơn thanh toán lưu trú (Check-out)"))
}

func (d *EmailDispatcher) alreadySent(eventID uuid.UUID) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.sent[eventID]
	return ok
}

func (d *EmailDispatcher) markSent(eventID uuid.UUID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.sent) > 10000 {
		d.sent = make(map[uuid.UUID]struct{})
	}
	d.sent[eventID] = struct{}{}
}

func (d *EmailDispatcher) handlePasswordReset(ctx context.Context, evt auditapp.OutboxEvent) error {
	if d.alreadySent(evt.ID) || d.sender == nil {
		return nil
	}
	var p PasswordResetEmailPayload
	if err := json.Unmarshal(evt.Payload, &p); err != nil {
		return fmt.Errorf("decode password reset payload: %w", err)
	}
	if d.cipher == nil {
		return fmt.Errorf("payload cipher is required to decrypt password reset token")
	}
	rawToken, err := d.cipher.Decrypt(p.EncryptedToken)
	if err != nil {
		return fmt.Errorf("decrypt password reset token: %w", err)
	}

	resetLink := fmt.Sprintf("%s/admin/reset-password?token=%s", d.frontendBaseURL, url.QueryEscape(rawToken))
	name := strings.TrimSpace(p.DisplayName)
	if name == "" {
		name = p.Email
	}

	textBody := fmt.Sprintf(
		"Xin chào %s,\n\n"+
			"Hệ thống nhận được yêu cầu đặt lại mật khẩu cho tài khoản của bạn.\n"+
			"Vui lòng mở đường dẫn sau để đặt mật khẩu mới (hiệu lực 30 phút, chỉ sử dụng 1 lần):\n\n"+
			"%s\n\n"+
			"Nếu bạn không yêu cầu đặt lại mật khẩu, vui lòng bỏ qua email này.\n",
		name, resetLink,
	)
	htmlBody := fmt.Sprintf(
		"<p>Xin chào <strong>%s</strong>,</p>"+
			"<p>Hệ thống nhận được yêu cầu đặt lại mật khẩu cho tài khoản của bạn.</p>"+
			"<p>Vui lòng bấm vào liên kết dưới đây để đặt mật khẩu mới (có hiệu lực 30 phút và chỉ dùng 1 lần):</p>"+
			"<p><a href=\"%s\">Đặt lại mật khẩu</a></p>"+
			"<p>Hoặc sao chép đường dẫn: <code>%s</code></p>"+
			"<p>Nếu bạn không thực hiện yêu cầu này, vui lòng bỏ qua email.</p>",
		html.EscapeString(name), html.EscapeString(resetLink), html.EscapeString(resetLink),
	)

	if err := d.sender.Send(ctx, smtp.Message{
		To:        p.Email,
		Subject:   "Đặt lại mật khẩu tài khoản — Hotel Management",
		TextBody:  textBody,
		HTMLBody:  htmlBody,
		MessageID: evt.ID.String(),
	}); err != nil {
		return err
	}
	d.markSent(evt.ID)
	return nil
}

func (d *EmailDispatcher) handleEmployeeActivation(ctx context.Context, evt auditapp.OutboxEvent) error {
	if d.alreadySent(evt.ID) || d.sender == nil {
		return nil
	}
	var p EmployeeActivationEmailPayload
	if err := json.Unmarshal(evt.Payload, &p); err != nil {
		return fmt.Errorf("decode employee activation payload: %w", err)
	}
	if d.cipher == nil {
		return fmt.Errorf("payload cipher is required to decrypt activation token")
	}
	rawToken, err := d.cipher.Decrypt(p.EncryptedToken)
	if err != nil {
		return fmt.Errorf("decrypt activation token: %w", err)
	}

	activateLink := fmt.Sprintf("%s/admin/activate?token=%s", d.frontendBaseURL, url.QueryEscape(rawToken))
	name := strings.TrimSpace(p.FullName)
	if name == "" {
		name = p.Email
	}

	textBody := fmt.Sprintf(
		"Xin chào %s,\n\n"+
			"Quản trị viên đã tạo tài khoản nhân viên cho bạn (Mã NV: %s, Chức danh: %s).\n"+
			"Vui lòng mở đường dẫn dưới đây để đặt mật khẩu lần đầu và kích hoạt tài khoản (hiệu lực 24 giờ, chỉ sử dụng 1 lần):\n\n"+
			"%s\n\n"+
			"Nếu liên kết đã hết hạn, vui lòng liên hệ Quản trị viên để được gửi lại email kích hoạt.\n",
		name, p.EmployeeCode, p.PositionTitle, activateLink,
	)
	htmlBody := fmt.Sprintf(
		"<p>Xin chào <strong>%s</strong>,</p>"+
			"<p>Quản trị viên đã tạo tài khoản nhân viên cho bạn (Mã NV: <strong>%s</strong>, Chức danh: <strong>%s</strong>).</p>"+
			"<p>Vui lòng bấm vào liên kết dưới đây để đặt mật khẩu lần đầu và kích hoạt tài khoản (có hiệu lực 24 giờ và chỉ dùng 1 lần):</p>"+
			"<p><a href=\"%s\">Kích hoạt tài khoản nhân viên</a></p>"+
			"<p>Hoặc sao chép đường dẫn: <code>%s</code></p>"+
			"<p>Nếu liên kết đã hết hạn, vui lòng liên hệ Quản trị viên để được gửi lại email kích hoạt.</p>",
		html.EscapeString(name),
		html.EscapeString(p.EmployeeCode),
		html.EscapeString(p.PositionTitle),
		html.EscapeString(activateLink),
		html.EscapeString(activateLink),
	)

	if err := d.sender.Send(ctx, smtp.Message{
		To:        p.Email,
		Subject:   "Kích hoạt tài khoản nhân viên — Hotel Management",
		TextBody:  textBody,
		HTMLBody:  htmlBody,
		MessageID: evt.ID.String(),
	}); err != nil {
		return err
	}
	d.markSent(evt.ID)
	return nil
}

func (d *EmailDispatcher) handleGenericNotification(defaultSubject string) HandlerFunc {
	return func(ctx context.Context, evt auditapp.OutboxEvent) error {
		if d.alreadySent(evt.ID) || d.sender == nil {
			return nil
		}
		var p GenericEmailPayload
		if err := json.Unmarshal(evt.Payload, &p); err != nil {
			return fmt.Errorf("decode notification payload: %w", err)
		}
		if strings.TrimSpace(p.Email) == "" {
			return nil
		}
		subject := strings.TrimSpace(p.Subject)
		if subject == "" {
			subject = defaultSubject
		}
		if err := d.sender.Send(ctx, smtp.Message{
			To:        p.Email,
			Subject:   subject,
			TextBody:  p.Summary,
			MessageID: evt.ID.String(),
		}); err != nil {
			return err
		}
		d.markSent(evt.ID)
		return nil
	}
}
