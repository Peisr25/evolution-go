package whatsmeow_service

import (
	"context"
	"errors"
	"testing"

	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	instance_repository "github.com/EvolutionAPI/evolution-go/pkg/instance/repository"
	"go.mau.fi/whatsmeow/types"
)

// mockInstanceRepository satisfies instance_repository.InstanceRepository for tests.
// Only GetInstanceByID is exercised; all other methods are no-ops.
type mockInstanceRepository struct {
	instance *instance_model.Instance
	err      error
}

func (m *mockInstanceRepository) GetInstanceByID(_ string) (*instance_model.Instance, error) {
	return m.instance, m.err
}
func (m *mockInstanceRepository) Create(_ instance_model.Instance) (*instance_model.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepository) GetConnectedInstanceByID(_ string) (*instance_model.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepository) GetInstanceByToken(_ string) (*instance_model.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepository) GetInstanceByName(_ string) (*instance_model.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepository) Update(_ *instance_model.Instance) error { return nil }
func (m *mockInstanceRepository) UpdateConnected(_ string, _ bool, _ string) error {
	return nil
}
func (m *mockInstanceRepository) UpdateQrcode(_ string, _ string) error               { return nil }
func (m *mockInstanceRepository) UpdateProxy(_ string, _ string) error                { return nil }
func (m *mockInstanceRepository) UpdateJid(_ string, _ string) error                  { return nil }
func (m *mockInstanceRepository) Delete(_ string) error                               { return nil }
func (m *mockInstanceRepository) GetAll(_ string) ([]*instance_model.Instance, error) { return nil, nil }
func (m *mockInstanceRepository) GetAllConnectedInstances() ([]*instance_model.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepository) GetAllConnectedInstancesByClientName(_ string) ([]*instance_model.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepository) GetAdvancedSettings(_ string) (*instance_model.AdvancedSettings, error) {
	return nil, nil
}
func (m *mockInstanceRepository) UpdateAdvancedSettings(_ string, _ *instance_model.AdvancedSettings) error {
	return nil
}

// Compile-time check that the mock fully satisfies the interface.
var _ instance_repository.InstanceRepository = (*mockInstanceRepository)(nil)

// mockPresenceSender records calls to SendPresence.
type mockPresenceSender struct {
	called   bool
	lastType types.Presence
	err      error
}

func (m *mockPresenceSender) SendPresence(_ context.Context, p types.Presence) error {
	m.called = true
	m.lastType = p
	return m.err
}

// --- tests ---

func TestHandlePresenceTick_InstanceNotFound(t *testing.T) {
	repo := &mockInstanceRepository{err: errors.New("record not found")}
	sender := &mockPresenceSender{}

	shouldStop, err := handlePresenceTick(context.Background(), "test-id", repo, sender)

	if !shouldStop {
		t.Error("expected shouldStop=true when instance is not found in DB")
	}
	if err == nil {
		t.Error("expected a non-nil error when instance is not found")
	}
	if sender.called {
		t.Error("SendPresence must not be called when instance is not found")
	}
}

func TestHandlePresenceTick_AlwaysOnlineFalse(t *testing.T) {
	repo := &mockInstanceRepository{instance: &instance_model.Instance{AlwaysOnline: false}}
	sender := &mockPresenceSender{}

	shouldStop, err := handlePresenceTick(context.Background(), "test-id", repo, sender)

	if !shouldStop {
		t.Error("expected shouldStop=true when AlwaysOnline=false")
	}
	if err != nil {
		t.Errorf("expected no error when AlwaysOnline=false, got: %v", err)
	}
	if sender.called {
		t.Error("SendPresence must not be called when AlwaysOnline=false")
	}
}

func TestHandlePresenceTick_AlwaysOnlineTrue_Success(t *testing.T) {
	repo := &mockInstanceRepository{instance: &instance_model.Instance{AlwaysOnline: true}}
	sender := &mockPresenceSender{}

	shouldStop, err := handlePresenceTick(context.Background(), "test-id", repo, sender)

	if shouldStop {
		t.Error("expected shouldStop=false when AlwaysOnline=true and SendPresence succeeds")
	}
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if !sender.called {
		t.Error("SendPresence must be called when AlwaysOnline=true")
	}
	if sender.lastType != types.PresenceAvailable {
		t.Errorf("expected PresenceAvailable, got: %v", sender.lastType)
	}
}

func TestHandlePresenceTick_AlwaysOnlineTrue_SendPresenceFails(t *testing.T) {
	repo := &mockInstanceRepository{instance: &instance_model.Instance{AlwaysOnline: true}}
	sender := &mockPresenceSender{err: errors.New("whatsapp unavailable")}

	shouldStop, err := handlePresenceTick(context.Background(), "test-id", repo, sender)

	// Even when SendPresence fails we keep the goroutine alive to retry next tick.
	if shouldStop {
		t.Error("expected shouldStop=false even when SendPresence fails (will retry next tick)")
	}
	if err == nil {
		t.Error("expected error when SendPresence fails")
	}
	if !sender.called {
		t.Error("SendPresence must have been attempted")
	}
}
