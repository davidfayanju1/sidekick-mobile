// Package messaging implements Phase 5: conversations, messages, system
// messages, receipts, images, read-only, blocks integration, sanitization,
// rate limiting. All writes go through this package; no client can insert
// system messages directly.
package messaging

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/audit"
	"github.com/sidekick/backend/internal/blobstore"
	"github.com/sidekick/backend/internal/storage"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrForbidden   = errors.New("forbidden")
	ErrBadRequest  = errors.New("bad request")
	ErrConflict    = errors.New("conflict")
	ErrRateLimited = errors.New("rate_limited")
	ErrChatFrozen  = errors.New("chat_frozen")
)

var validSystemEvents = map[string]bool{
	"accepted": true, "marked_complete": true, "payment_released": true,
	"cancelled": true, "dispute_raised": true, "dispute_resolved": true,
	"refunded": true, "auto_release_scheduled": true,
}

// Service handles messaging operations.
type Service struct {
	pool     *pgxpool.Pool
	signer   *storage.Store
	blobs    *blobstore.MemoryBlobStore
	rateMu   sync.Mutex
	rateHits map[string][]time.Time // userID -> timestamps
}

func New(pool *pgxpool.Pool, signer *storage.Store, blobs *blobstore.MemoryBlobStore) *Service {
	if blobs == nil {
		blobs = blobstore.NewMemoryBlobStore()
	}
	if signer == nil {
		signer = storage.NewStore("")
	}
	return &Service{
		pool:     pool,
		signer:   signer,
		blobs:    blobs,
		rateHits: map[string][]time.Time{},
	}
}

func (s *Service) Blobs() *blobstore.MemoryBlobStore { return s.blobs }
func (s *Service) Signer() *storage.Store { return s.signer }

// allow checks 30/min per user; returns false if over limit.
func (s *Service) allow(userID string) bool {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	now := time.Now()
	cut := now.Add(-time.Minute)
	hits := s.rateHits[userID][:0]
	for _, t := range s.rateHits[userID] {
		if t.After(cut) {
			hits = append(hits, t)
		}
	}
	if len(hits) >= 30 {
		s.rateHits[userID] = hits
		return false
	}
	s.rateHits[userID] = append(hits, now)
	return true
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

func sanitize(body string) string {
	stripped := tagRE.ReplaceAllString(body, "")
	stripped = strings.TrimSpace(stripped)
	return stripped
}

type Conversation struct {
	ID                 string     `json:"id"`
	TaskID             string     `json:"task_id"`
	PosterID           string     `json:"poster_id"`
	WorkerID           string     `json:"worker_id"`
	LastMessageAt      *time.Time `json:"last_message_at,omitempty"`
	LastMessagePreview *string    `json:"last_message_preview,omitempty"`
	ReadOnly           bool       `json:"read_only"`
	ReadOnlyAt         *time.Time `json:"read_only_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type Message struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	SenderID       *string    `json:"sender_id"`
	Type           string     `json:"type"`
	Body           string     `json:"body"`
	SystemEvent    *string    `json:"system_event"`
	AttachmentURL  *string    `json:"attachment_url"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// isParticipant checks if user is in conversation.
func (s *Service) isParticipant(ctx context.Context, userID, convID string) (bool, *Conversation, error) {
	var c Conversation
	err := s.pool.QueryRow(ctx,
		`SELECT id::text, task_id::text, poster_id::text, worker_id::text, last_message_at, last_message_preview, read_only, read_only_at, created_at
		 FROM conversations WHERE id=$1::uuid`, convID).Scan(
		&c.ID, &c.TaskID, &c.PosterID, &c.WorkerID, &c.LastMessageAt, &c.LastMessagePreview, &c.ReadOnly, &c.ReadOnlyAt, &c.CreatedAt)
	if err != nil {
		return false, nil, ErrNotFound
	}
	if userID != c.PosterID && userID != c.WorkerID {
		return false, nil, ErrForbidden
	}
	return true, &c, nil
}

// CheckParticipant is the public version for HTTP handlers.
func (s *Service) CheckParticipant(ctx context.Context, userID, convID string) error {
	ok, _, err := s.isParticipant(ctx, userID, convID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *Service) isBlocked(ctx context.Context, a, b string) (bool, error) {
	var cnt int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM blocks WHERE (blocker_id=$1::uuid AND blocked_id=$2::uuid) OR (blocker_id=$2::uuid AND blocked_id=$1::uuid)`, a, b).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// ListConversations returns conversations for user ordered by last_message_at desc.
func (s *Service) ListConversations(ctx context.Context, userID string) ([]Conversation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, task_id::text, poster_id::text, worker_id::text, last_message_at, last_message_preview, read_only, read_only_at, created_at
		 FROM conversations WHERE poster_id=$1::uuid OR worker_id=$1::uuid
		 ORDER BY COALESCE(last_message_at, created_at) DESC, created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.TaskID, &c.PosterID, &c.WorkerID, &c.LastMessageAt, &c.LastMessagePreview, &c.ReadOnly, &c.ReadOnlyAt, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if out == nil {
		out = []Conversation{}
	}
	return out, rows.Err()
}

// ListMessages returns paginated messages for conversation, most-recent-first.
func (s *Service) ListMessages(ctx context.Context, userID, convID string, cursor string, limit int) ([]Message, *string, error) {
	ok, _, err := s.isParticipant(ctx, userID, convID)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, ErrForbidden
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	var cursorTime *time.Time
	var cursorID string
	if cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(cursor)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: invalid cursor", ErrBadRequest)
		}
		parts := strings.SplitN(string(decoded), "|", 2)
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("%w: invalid cursor", ErrBadRequest)
		}
		t, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return nil, nil, fmt.Errorf("%w: invalid cursor time", ErrBadRequest)
		}
		cursorTime = &t
		cursorID = parts[1]
	}
	query := `SELECT id::text, conversation_id::text, sender_id::text, type, body, system_event, attachment_url, delivered_at, read_at, created_at
			  FROM messages WHERE conversation_id=$1::uuid`
	args := []any{convID}
	if cursorTime != nil {
		query += ` AND (created_at < $2 OR (created_at = $2 AND id::text < $3))`
		args = append(args, *cursorTime, cursorID)
	}
	query += ` ORDER BY created_at DESC, id::text DESC LIMIT ` + fmt.Sprintf("%d", limit+1)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Type, &m.Body, &m.SystemEvent, &m.AttachmentURL, &m.DeliveredAt, &m.ReadAt, &m.CreatedAt); err != nil {
			return nil, nil, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	var next *string
	if len(msgs) > limit {
		last := msgs[limit-1]
		cur := base64.StdEncoding.EncodeToString([]byte(last.CreatedAt.Format(time.RFC3339Nano) + "|" + last.ID))
		next = &cur
		msgs = msgs[:limit]
	}
	if msgs == nil {
		msgs = []Message{}
	}
	return msgs, next, nil
}

// SendText sends a text message.
func (s *Service) SendText(ctx context.Context, senderID, convID, body string) (*Message, error) {
	if !s.allow(senderID) {
		return nil, ErrRateLimited
	}
	body = sanitize(body)
	if body == "" {
		return nil, fmt.Errorf("%w: body required", ErrBadRequest)
	}
	if len([]rune(body)) > 2000 {
		return nil, fmt.Errorf("%w: body max 2000 chars", ErrBadRequest)
	}
	ok, conv, err := s.isParticipant(ctx, senderID, convID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if conv.ReadOnly {
		return nil, ErrChatFrozen
	}
	other := conv.PosterID
	if senderID == conv.PosterID {
		other = conv.WorkerID
	}
	blocked, err := s.isBlocked(ctx, senderID, other)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrForbidden
	}
	var m Message
	err = s.pool.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, sender_id, type, body) VALUES ($1::uuid,$2::uuid,'text',$3) RETURNING id::text, conversation_id::text, sender_id::text, type, body, system_event, attachment_url, delivered_at, read_at, created_at`,
		convID, senderID, body).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Type, &m.Body, &m.SystemEvent, &m.AttachmentURL, &m.DeliveredAt, &m.ReadAt, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE conversations SET last_message_at=now(), last_message_preview=$2 WHERE id=$1::uuid`, convID, body[:min(100, len(body))])
	return &m, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MarkDelivered sets delivered_at if caller is recipient (not sender).
func (s *Service) MarkDelivered(ctx context.Context, userID, messageID string) error {
	var senderID *string
	var posterID, workerID string
	err := s.pool.QueryRow(ctx,
		`SELECT m.sender_id::text, c.poster_id::text, c.worker_id::text
		 FROM messages m JOIN conversations c ON c.id=m.conversation_id WHERE m.id=$1::uuid`, messageID).Scan(&senderID, &posterID, &workerID)
	if err != nil {
		return ErrNotFound
	}
	if userID != posterID && userID != workerID {
		return ErrForbidden
	}
	if senderID != nil && *senderID == userID {
		return fmt.Errorf("%w: sender cannot mark own message delivered", ErrForbidden)
	}
	_, err = s.pool.Exec(ctx, `UPDATE messages SET delivered_at=COALESCE(delivered_at, now()) WHERE id=$1::uuid`, messageID)
	return err
}

// MarkRead sets read_at if caller is recipient.
func (s *Service) MarkRead(ctx context.Context, userID, messageID string) error {
	var senderID *string
	var posterID, workerID string
	err := s.pool.QueryRow(ctx,
		`SELECT m.sender_id::text, c.poster_id::text, c.worker_id::text
		 FROM messages m JOIN conversations c ON c.id=m.conversation_id WHERE m.id=$1::uuid`, messageID).Scan(&senderID, &posterID, &workerID)
	if err != nil {
		return ErrNotFound
	}
	if userID != posterID && userID != workerID {
		return ErrForbidden
	}
	if senderID != nil && *senderID == userID {
		return fmt.Errorf("%w: sender cannot mark own message read", ErrForbidden)
	}
	_, err = s.pool.Exec(ctx, `UPDATE messages SET read_at=COALESCE(read_at, now()), delivered_at=COALESCE(delivered_at, now()) WHERE id=$1::uuid`, messageID)
	return err
}

// InsertSystemMessage is the ONLY place system messages are created.
func InsertSystemMessage(ctx context.Context, pool *pgxpool.Pool, conversationID, systemEvent, body string) (string, error) {
	if !validSystemEvents[systemEvent] {
		return "", fmt.Errorf("%w: invalid system_event %s", ErrBadRequest, systemEvent)
	}
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("%w: body required", ErrBadRequest)
	}
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, sender_id, type, body, system_event) VALUES ($1::uuid,NULL,'system',$2,$3) RETURNING id::text`,
		conversationID, body, systemEvent).Scan(&id)
	if err != nil {
		return "", err
	}
	_, _ = pool.Exec(ctx, `UPDATE conversations SET last_message_at=now(), last_message_preview=$2 WHERE id=$1::uuid`, conversationID, body[:min(100, len(body))])
	return id, nil
}

// InsertSystemMessageTx is the transactional version for use inside offers accept.
func InsertSystemMessageTx(ctx context.Context, tx pgx.Tx, conversationID, systemEvent, body string) (string, error) {
	if !validSystemEvents[systemEvent] {
		return "", fmt.Errorf("%w: invalid system_event %s", ErrBadRequest, systemEvent)
	}
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("%w: body required", ErrBadRequest)
	}
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, sender_id, type, body, system_event) VALUES ($1::uuid,NULL,'system',$2,$3) RETURNING id::text`,
		conversationID, body, systemEvent).Scan(&id)
	if err != nil {
		return "", err
	}
	_, _ = tx.Exec(ctx, `UPDATE conversations SET last_message_at=now(), last_message_preview=$2 WHERE id=$1::uuid`, conversationID, body[:min(100, len(body))])
	return id, nil
}

// Image handling

func (s *Service) GrantImageUpload(conversationID, contentType string) (storage.SignedURL, error) {
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		return storage.SignedURL{}, fmt.Errorf("%w: content_type must be image/jpeg, image/png or image/webp", ErrBadRequest)
	}
	ext := ".jpg"
	if contentType == "image/png" {
		ext = ".png"
	} else if contentType == "image/webp" {
		ext = ".webp"
	}
	object := conversationID + "/" + uuid.NewString() + ext
	return s.signer.MintUpload(storage.BucketChatImages, object, 15*time.Minute), nil
}

func (s *Service) SendImage(ctx context.Context, senderID, convID string, signed storage.SignedURL, data []byte) (*Message, error) {
	if !s.allow(senderID) {
		return nil, ErrRateLimited
	}
	if len(data) > 5*1024*1024 {
		return nil, fmt.Errorf("%w: image max 5MB", ErrBadRequest)
	}
	if err := s.signer.Verify("PUT", signed); err != nil {
		return nil, fmt.Errorf("%w: invalid upload signature", ErrBadRequest)
	}
	if signed.Bucket != storage.BucketChatImages {
		return nil, fmt.Errorf("%w: wrong bucket", ErrBadRequest)
	}
	ok, conv, err := s.isParticipant(ctx, senderID, convID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if conv.ReadOnly {
		return nil, ErrChatFrozen
	}
	other := conv.PosterID
	if senderID == conv.PosterID {
		other = conv.WorkerID
	}
	blocked, err := s.isBlocked(ctx, senderID, other)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrForbidden
	}
	key := string(signed.Bucket) + "/" + signed.Object
	s.blobs.Put(key, data, "image/jpeg")
	download := s.signer.MintDownload(storage.BucketChatImages, signed.Object, 24*time.Hour)
	attachment := "/chat-images/" + signed.Object + "?expires=" + download.ExpiresAt.Format(time.RFC3339) + "&sig=" + download.Signature
	var m Message
	err = s.pool.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, sender_id, type, body, attachment_url) VALUES ($1::uuid,$2::uuid,'image','[image]',$3) RETURNING id::text, conversation_id::text, sender_id::text, type, body, system_event, attachment_url, delivered_at, read_at, created_at`,
		convID, senderID, attachment).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Type, &m.Body, &m.SystemEvent, &m.AttachmentURL, &m.DeliveredAt, &m.ReadAt, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE conversations SET last_message_at=now(), last_message_preview='[image]' WHERE id=$1::uuid`, convID)
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &senderID, Action: "message.image", EntityType: "message", EntityID: m.ID})
	return &m, nil
}

func (s *Service) GetImage(ctx context.Context, userID, object, sig, expires string) ([]byte, error) {
	exp, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid expires", ErrBadRequest)
	}
	signed := storage.SignedURL{Bucket: storage.BucketChatImages, Object: object, ExpiresAt: exp, Signature: sig}
	if err := s.signer.Verify("GET", signed); err != nil {
		return nil, ErrForbidden
	}
	// Verify user is participant of conversation that owns the object (object prefix is convID)
	parts := strings.SplitN(object, "/", 2)
	if len(parts) != 2 {
		return nil, ErrNotFound
	}
	convID := parts[0]
	ok, _, err := s.isParticipant(ctx, userID, convID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	key := string(storage.BucketChatImages) + "/" + object
	data, ok := s.blobs.Get(key)
	if !ok {
		return nil, ErrNotFound
	}
	return data, nil
}
