package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"net/textproto"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

// fakeSender is a hand-rolled Sender fake (no mocking library, per repo
// convention).
type fakeSender struct {
	err   error
	calls int
	last  Message
}

func (f *fakeSender) Send(_ context.Context, msg Message) error {
	f.calls++
	f.last = msg
	return f.err
}

// fakeTaskEnqueuer is a hand-rolled TaskEnqueuer fake.
type fakeTaskEnqueuer struct {
	err   error
	tasks []*asynq.Task
}

func (f *fakeTaskEnqueuer) EnqueueContext(_ context.Context, task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.tasks = append(f.tasks, task)
	return &asynq.TaskInfo{ID: "task-1", Queue: QueueName}, nil
}

func TestNewSendEmailTask(t *testing.T) {
	msg := validMessage()
	msg.HTMLBody = "<p>Привет</p>"

	task, err := NewSendEmailTask(msg)
	if err != nil {
		t.Fatalf("NewSendEmailTask returned error: %v", err)
	}
	if task.Type() != TaskTypeSendEmail || TaskTypeSendEmail != "email:send" {
		t.Errorf("task type = %q", task.Type())
	}

	var decoded Message
	if err := json.Unmarshal(task.Payload(), &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decoded != msg {
		t.Errorf("payload round-trip mismatch: got %+v, want %+v", decoded, msg)
	}
}

func TestSendEmailTaskOptions(t *testing.T) {
	var gotQueue string
	var gotMaxRetry int
	var gotTimeout time.Duration
	for _, opt := range sendEmailTaskOptions() {
		switch opt.Type() {
		case asynq.QueueOpt:
			gotQueue = opt.Value().(string)
		case asynq.MaxRetryOpt:
			gotMaxRetry = opt.Value().(int)
		case asynq.TimeoutOpt:
			gotTimeout = opt.Value().(time.Duration)
		case asynq.RetentionOpt:
			t.Error("email tasks must not set Retention (payloads contain tokens)")
		}
	}
	if gotQueue != "email" || QueueName != "email" {
		t.Errorf("queue = %q, want email", gotQueue)
	}
	if gotMaxRetry != 8 {
		t.Errorf("MaxRetry = %d, want 8", gotMaxRetry)
	}
	if gotTimeout != time.Minute {
		t.Errorf("Timeout = %v, want 1m", gotTimeout)
	}
}

func TestNewSendEmailTask_InvalidMessage(t *testing.T) {
	msg := validMessage()
	msg.To = ""
	if _, err := NewSendEmailTask(msg); !errors.Is(err, ErrInvalidRecipient) {
		t.Fatalf("expected ErrInvalidRecipient, got %v", err)
	}
}

func payloadTask(t *testing.T, msg Message) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return asynq.NewTask(TaskTypeSendEmail, payload)
}

func TestHandleTask_Success(t *testing.T) {
	sender := &fakeSender{}
	h := NewHandler(sender)
	msg := validMessage()

	if err := h.HandleTask(context.Background(), payloadTask(t, msg)); err != nil {
		t.Fatalf("HandleTask returned error: %v", err)
	}
	if sender.calls != 1 || sender.last != msg {
		t.Errorf("sender got calls=%d msg=%+v", sender.calls, sender.last)
	}
}

func TestHandleTask_MalformedJSON(t *testing.T) {
	sender := &fakeSender{}
	err := NewHandler(sender).HandleTask(context.Background(), asynq.NewTask(TaskTypeSendEmail, []byte("{not json")))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected SkipRetry, got %v", err)
	}
	if sender.calls != 0 {
		t.Error("sender must not be called")
	}
}

func TestHandleTask_InvalidMessage(t *testing.T) {
	sender := &fakeSender{}
	msg := validMessage()
	msg.Subject = ""
	err := NewHandler(sender).HandleTask(context.Background(), payloadTask(t, msg))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected SkipRetry, got %v", err)
	}
	if sender.calls != 0 {
		t.Error("sender must not be called for an invalid message")
	}
}

func TestHandleTask_ErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		skipRetry bool
	}{
		{"permanent 550", &textproto.Error{Code: 550, Msg: "mailbox unavailable"}, true},
		{"wrapped permanent 554", errors.Join(errors.New("rcpt to"), &textproto.Error{Code: 554, Msg: "rejected"}), true},
		{"transient 451", &textproto.Error{Code: 451, Msg: "try again later"}, false},
		{"network error", errors.New("dial tcp: connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := validMessage()
			err := NewHandler(&fakeSender{err: tt.err}).HandleTask(context.Background(), payloadTask(t, msg))
			if err == nil {
				t.Fatal("expected error")
			}
			if got := errors.Is(err, asynq.SkipRetry); got != tt.skipRetry {
				t.Errorf("SkipRetry = %v, want %v (err=%v)", got, tt.skipRetry, err)
			}
		})
	}
}

func TestQueueEnqueue(t *testing.T) {
	fake := &fakeTaskEnqueuer{}
	msg := validMessage()

	if err := NewQueue(fake).Enqueue(context.Background(), msg); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	if len(fake.tasks) != 1 {
		t.Fatalf("expected 1 enqueued task, got %d", len(fake.tasks))
	}
	if fake.tasks[0].Type() != TaskTypeSendEmail {
		t.Errorf("task type = %q", fake.tasks[0].Type())
	}
	var decoded Message
	if err := json.Unmarshal(fake.tasks[0].Payload(), &decoded); err != nil || decoded != msg {
		t.Errorf("payload mismatch: %+v, err=%v", decoded, err)
	}
}

func TestQueueEnqueue_InvalidMessage(t *testing.T) {
	fake := &fakeTaskEnqueuer{}
	msg := validMessage()
	msg.TextBody = ""

	if err := NewQueue(fake).Enqueue(context.Background(), msg); !errors.Is(err, ErrEmptyTextBody) {
		t.Fatalf("expected ErrEmptyTextBody, got %v", err)
	}
	if len(fake.tasks) != 0 {
		t.Error("nothing must be enqueued for an invalid message")
	}
}

func TestQueueEnqueue_EnqueuerError(t *testing.T) {
	redisDown := errors.New("redis down")
	err := NewQueue(&fakeTaskEnqueuer{err: redisDown}).Enqueue(context.Background(), validMessage())
	if !errors.Is(err, redisDown) {
		t.Fatalf("expected enqueuer error to be propagated, got %v", err)
	}
}
