package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Status string
type Phase string

const (
	StatusIdle    Status = "idle"
	StatusRunning Status = "running"
	StatusPaused  Status = "paused"

	PhaseSitting  Phase = "sitting"
	PhaseStanding Phase = "standing"
	PhaseRest     Phase = "rest"
)

var phaseOrder = []Phase{PhaseSitting, PhaseStanding, PhaseRest}

var phaseDurations = map[Phase]time.Duration{
	PhaseSitting:  40 * time.Minute,
	PhaseStanding: 15 * time.Minute,
	PhaseRest:     5 * time.Minute,
}

type PhaseRecord struct {
	Phase      Phase     `json:"phase"`
	StartedAt  time.Time `json:"startedAt"`
	EndedAt    time.Time `json:"endedAt"`
	PlannedSec float64   `json:"plannedSec"`
	ActualSec  float64   `json:"actualSec"`
	LateSec    float64   `json:"lateSec"`
	LateMin    float64   `json:"lateMin"`
	Confirmed  bool      `json:"confirmed"`
}

type Session struct {
	ID              string        `json:"id"`
	Status          Status        `json:"status"`
	Phase           Phase         `json:"phase"`
	PhaseBeganAt    time.Time     `json:"phaseBeganAt"`
	PhaseStartedAt  time.Time     `json:"phaseStartedAt"`
	AccumulatedSec  float64       `json:"accumulatedSec"`
	AwaitingAdvance bool          `json:"awaitingAdvance"`
	Phases          []PhaseRecord `json:"phases"`
	PausedSec       float64       `json:"pausedSec"`
	PauseCount      int           `json:"pauseCount"`
	PausedAt        *time.Time    `json:"pausedAt,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

type SessionArchive struct {
	ID        string         `json:"id"`
	StartedAt time.Time      `json:"startedAt"`
	EndedAt   time.Time      `json:"endedAt"`
	Phases    []PhaseRecord  `json:"phases"`
	Totals    SessionTotals  `json:"totals"`
	Summary   SessionSummary `json:"summary"`
}

type SessionTotals struct {
	DurationSec     float64        `json:"durationSec"`
	DurationMin     float64        `json:"durationMin"`
	WorkSec         float64        `json:"workSec"`
	RestSec         float64        `json:"restSec"`
	SittingSec      float64        `json:"sittingSec"`
	StandingSec     float64        `json:"standingSec"`
	LateSec         float64        `json:"lateSec"`
	LateMin         float64        `json:"lateMin"`
	PausedSec       float64        `json:"pausedSec"`
	PauseCount      int            `json:"pauseCount"`
	TransitionCount int            `json:"transitionCount"`
	PhaseCounts     map[string]int `json:"phaseCounts"`
	AvgLateSec      float64        `json:"avgLateSec"`
}

type SessionSummary struct {
	StartedAt      string  `json:"startedAt"`
	EndedAt        string  `json:"endedAt"`
	TotalHuman     string  `json:"totalHuman"`
	WorkHuman      string  `json:"workHuman"`
	RestHuman      string  `json:"restHuman"`
	LateHuman      string  `json:"lateHuman"`
	WorkedHours    float64 `json:"workedHours"`
	Transitions    int     `json:"transitions"`
	OnTimeAdvances int     `json:"onTimeAdvances"`
	LateAdvances   int     `json:"lateAdvances"`
}

type SessionListItem struct {
	ID          string  `json:"id"`
	StartedAt   string  `json:"startedAt"`
	EndedAt     string  `json:"endedAt"`
	TotalHuman  string  `json:"totalHuman"`
	WorkHuman   string  `json:"workHuman"`
	LateHuman   string  `json:"lateHuman"`
	WorkedHours float64 `json:"workedHours"`
	Transitions int     `json:"transitions"`
	LateMin     float64 `json:"lateMin"`
}

type PublicState struct {
	ID              string  `json:"id,omitempty"`
	Status          Status  `json:"status"`
	Phase           Phase   `json:"phase,omitempty"`
	NextPhase       Phase   `json:"nextPhase,omitempty"`
	PhaseDuration   int     `json:"phaseDurationSec"`
	RemainingSec    float64 `json:"remainingSec"`
	ElapsedSec      float64 `json:"elapsedSec"`
	OvertimeSec     float64 `json:"overtimeSec"`
	AwaitingAdvance bool    `json:"awaitingAdvance"`
	TransitionCount int     `json:"transitionCount"`
	StartedAt       string  `json:"startedAt,omitempty"`
	UpdatedAt       string  `json:"updatedAt"`
}

type Store struct {
	mu      sync.Mutex
	db      *pgxpool.Pool
	session *Session
}

func NewStore(ctx context.Context, db *pgxpool.Pool) (*Store, error) {
	s := &Store{db: db}
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.Ping(ctx)
}

func (s *Store) CloseDB() {
	s.db.Close()
}

func (s *Store) load(ctx context.Context) error {
	var raw []byte
	err := s.db.QueryRow(ctx, `SELECT data FROM live_session WHERE id = 1`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var sess Session
	if err := json.Unmarshal(raw, &sess); err != nil {
		return err
	}
	if sess.Phases == nil {
		sess.Phases = []PhaseRecord{}
	}
	if sess.PhaseBeganAt.IsZero() {
		sess.PhaseBeganAt = sess.PhaseStartedAt
	}
	s.session = &sess
	return nil
}

func (s *Store) saveLocked() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.session == nil {
		_, err := s.db.Exec(ctx, `DELETE FROM live_session WHERE id = 1`)
		return err
	}

	s.session.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(s.session)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
INSERT INTO live_session (id, data, updated_at)
VALUES (1, $1::jsonb, NOW())
ON CONFLICT (id) DO UPDATE
SET data = EXCLUDED.data, updated_at = NOW()
`, data)
	return err
}

func nextPhase(p Phase) Phase {
	for i, ph := range phaseOrder {
		if ph == p {
			return phaseOrder[(i+1)%len(phaseOrder)]
		}
	}
	return PhaseSitting
}

func (s *Store) currentElapsedLocked(now time.Time) float64 {
	sess := s.session
	elapsed := sess.AccumulatedSec
	if sess.Status == StatusRunning {
		elapsed += now.Sub(sess.PhaseStartedAt).Seconds()
	}
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func (s *Store) syncLocked(now time.Time) {
	if s.session == nil || s.session.Status != StatusRunning || s.session.AwaitingAdvance {
		return
	}
	dur := phaseDurations[s.session.Phase].Seconds()
	if s.currentElapsedLocked(now) >= dur {
		s.session.AwaitingAdvance = true
	}
}

func (s *Store) publicLocked(now time.Time) (PublicState, bool) {
	if s.session == nil {
		return PublicState{
			Status:    StatusIdle,
			UpdatedAt: now.UTC().Format(time.RFC3339Nano),
		}, false
	}

	wasAwaiting := s.session.AwaitingAdvance
	s.syncLocked(now)

	sess := s.session
	dur := phaseDurations[sess.Phase].Seconds()
	elapsed := s.currentElapsedLocked(now)
	remaining := dur - elapsed
	overtime := 0.0
	if remaining < 0 {
		overtime = -remaining
	}

	state := PublicState{
		ID:              sess.ID,
		Status:          sess.Status,
		Phase:           sess.Phase,
		PhaseDuration:   int(dur),
		RemainingSec:    remaining,
		ElapsedSec:      elapsed,
		OvertimeSec:     overtime,
		AwaitingAdvance: sess.AwaitingAdvance || remaining <= 0,
		TransitionCount: len(sess.Phases),
		StartedAt:       sess.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:       now.UTC().Format(time.RFC3339Nano),
	}
	if state.AwaitingAdvance {
		state.NextPhase = nextPhase(sess.Phase)
		s.session.AwaitingAdvance = true
	}
	flipped := !wasAwaiting && s.session.AwaitingAdvance
	return state, flipped
}

func (s *Store) View() PublicState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, flipped := s.publicLocked(time.Now())
	if flipped {
		if err := s.saveLocked(); err != nil {
			log.Printf("persist awaiting flip: %v", err)
		}
	}
	return state
}

func (s *Store) Snapshot() PublicState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, _ := s.publicLocked(time.Now())
	if err := s.saveLocked(); err != nil {
		log.Printf("snapshot save: %v", err)
	}
	return state
}

func (s *Store) Start() (PublicState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()

	if s.session != nil && s.session.Status == StatusPaused {
		if s.session.PausedAt != nil {
			s.session.PausedSec += now.Sub(*s.session.PausedAt).Seconds()
		}
		s.session.Status = StatusRunning
		s.session.PhaseStartedAt = now
		s.session.PausedAt = nil
		if err := s.saveLocked(); err != nil {
			return PublicState{}, err
		}
		state, _ := s.publicLocked(now)
		return state, nil
	}

	if s.session != nil && s.session.Status == StatusRunning {
		state, _ := s.publicLocked(now)
		return state, nil
	}

	s.session = &Session{
		ID:              fmt.Sprintf("%d", now.UnixNano()),
		Status:          StatusRunning,
		Phase:           PhaseSitting,
		PhaseBeganAt:    now,
		PhaseStartedAt:  now,
		AccumulatedSec:  0,
		AwaitingAdvance: false,
		Phases:          []PhaseRecord{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.saveLocked(); err != nil {
		return PublicState{}, err
	}
	state, _ := s.publicLocked(now)
	return state, nil
}

func (s *Store) Stop() (PublicState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()

	if s.session == nil || s.session.Status != StatusRunning {
		state, _ := s.publicLocked(now)
		return state, nil
	}

	s.syncLocked(now)
	s.session.AccumulatedSec = s.currentElapsedLocked(now)
	s.session.Status = StatusPaused
	paused := now
	s.session.PausedAt = &paused
	s.session.PauseCount++
	if err := s.saveLocked(); err != nil {
		return PublicState{}, err
	}
	state, _ := s.publicLocked(now)
	return state, nil
}

func makePhaseRecord(phase Phase, began, ended time.Time, actual float64, confirmed bool) PhaseRecord {
	planned := phaseDurations[phase].Seconds()
	late := actual - planned
	if late < 0 {
		late = 0
	}
	return PhaseRecord{
		Phase:      phase,
		StartedAt:  began.UTC(),
		EndedAt:    ended.UTC(),
		PlannedSec: planned,
		ActualSec:  round1(actual),
		LateSec:    round1(late),
		LateMin:    round1(late / 60),
		Confirmed:  confirmed,
	}
}

func (s *Store) Advance() (PublicState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()

	if s.session == nil {
		state, _ := s.publicLocked(now)
		return state, nil
	}

	s.syncLocked(now)
	elapsed := s.currentElapsedLocked(now)
	dur := phaseDurations[s.session.Phase].Seconds()
	if elapsed < dur && !s.session.AwaitingAdvance {
		state, _ := s.publicLocked(now)
		return state, nil
	}

	from := s.session.Phase
	to := nextPhase(from)
	s.session.Phases = append(s.session.Phases, makePhaseRecord(
		from, s.session.PhaseBeganAt, now, elapsed, true,
	))
	s.session.Phase = to
	s.session.PhaseBeganAt = now
	s.session.PhaseStartedAt = now
	s.session.AccumulatedSec = 0
	s.session.AwaitingAdvance = false
	if s.session.Status == StatusPaused {
		paused := now
		s.session.PausedAt = &paused
	}

	if err := s.saveLocked(); err != nil {
		return PublicState{}, err
	}
	state, _ := s.publicLocked(now)
	return state, nil
}

func (s *Store) Close() (PublicState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()

	if s.session != nil {
		if s.session.Status == StatusPaused && s.session.PausedAt != nil {
			s.session.PausedSec += now.Sub(*s.session.PausedAt).Seconds()
			s.session.PausedAt = nil
		}
		elapsed := s.currentElapsedLocked(now)
		if s.session.Status == StatusRunning {
			s.session.AccumulatedSec = elapsed
		}
		s.session.Phases = append(s.session.Phases, makePhaseRecord(
			s.session.Phase, s.session.PhaseBeganAt, now, elapsed, false,
		))
		if err := s.archiveLocked(now); err != nil {
			return PublicState{}, err
		}
	}

	s.session = nil
	if err := s.saveLocked(); err != nil {
		return PublicState{}, err
	}
	state, _ := s.publicLocked(now)
	return state, nil
}

func (s *Store) archiveLocked(endedAt time.Time) error {
	sess := s.session
	if sess == nil {
		return nil
	}

	archive := buildArchive(sess, endedAt)
	data, err := json.Marshal(archive)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = s.db.Exec(ctx, `
INSERT INTO session_archives (
  id, started_at, ended_at,
  started_label, ended_label, total_human, work_human, late_human,
  worked_hours, transitions, late_min, archive
) VALUES (
  $1, $2, $3,
  $4, $5, $6, $7, $8,
  $9, $10, $11, $12::jsonb
)
ON CONFLICT (id) DO NOTHING
`,
		archive.ID,
		archive.StartedAt,
		archive.EndedAt,
		archive.Summary.StartedAt,
		archive.Summary.EndedAt,
		archive.Summary.TotalHuman,
		archive.Summary.WorkHuman,
		archive.Summary.LateHuman,
		archive.Summary.WorkedHours,
		archive.Summary.Transitions,
		archive.Totals.LateMin,
		data,
	)
	return err
}

func buildArchive(sess *Session, endedAt time.Time) SessionArchive {
	totals := SessionTotals{
		PhaseCounts: map[string]int{},
	}
	onTime, lateAdvances := 0, 0

	for _, p := range sess.Phases {
		totals.PhaseCounts[string(p.Phase)]++
		totals.LateSec += p.LateSec
		switch p.Phase {
		case PhaseSitting:
			totals.SittingSec += p.ActualSec
			totals.WorkSec += p.ActualSec
		case PhaseStanding:
			totals.StandingSec += p.ActualSec
			totals.WorkSec += p.ActualSec
		case PhaseRest:
			totals.RestSec += p.ActualSec
		}
		if p.Confirmed {
			totals.TransitionCount++
			if p.LateSec > 0 {
				lateAdvances++
			} else {
				onTime++
			}
		}
	}

	duration := endedAt.Sub(sess.CreatedAt).Seconds()
	if duration < 0 {
		duration = 0
	}
	totals.DurationSec = round1(duration)
	totals.DurationMin = round1(duration / 60)
	totals.WorkSec = round1(totals.WorkSec)
	totals.RestSec = round1(totals.RestSec)
	totals.SittingSec = round1(totals.SittingSec)
	totals.StandingSec = round1(totals.StandingSec)
	totals.LateSec = round1(totals.LateSec)
	totals.LateMin = round1(totals.LateSec / 60)
	totals.PausedSec = round1(sess.PausedSec)
	totals.PauseCount = sess.PauseCount
	if totals.TransitionCount > 0 {
		totals.AvgLateSec = round1(totals.LateSec / float64(totals.TransitionCount))
	}

	return SessionArchive{
		ID:        sess.ID,
		StartedAt: sess.CreatedAt.UTC(),
		EndedAt:   endedAt.UTC(),
		Phases:    sess.Phases,
		Totals:    totals,
		Summary: SessionSummary{
			StartedAt:      formatTR(sess.CreatedAt),
			EndedAt:        formatTR(endedAt),
			TotalHuman:     humanDuration(duration),
			WorkHuman:      humanDuration(totals.WorkSec),
			RestHuman:      humanDuration(totals.RestSec),
			LateHuman:      humanDuration(totals.LateSec),
			WorkedHours:    round2(totals.WorkSec / 3600),
			Transitions:    totals.TransitionCount,
			OnTimeAdvances: onTime,
			LateAdvances:   lateAdvances,
		},
	}
}

func (s *Store) ListSessions() ([]SessionListItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.db.Query(ctx, `
SELECT id, started_label, ended_label, total_human, work_human, late_human,
       worked_hours, transitions, late_min
FROM session_archives
ORDER BY ended_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SessionListItem, 0)
	for rows.Next() {
		var item SessionListItem
		if err := rows.Scan(
			&item.ID,
			&item.StartedAt,
			&item.EndedAt,
			&item.TotalHuman,
			&item.WorkHuman,
			&item.LateHuman,
			&item.WorkedHours,
			&item.Transitions,
			&item.LateMin,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetSession(id string) (*SessionArchive, error) {
	if id == "" || id == "." || id == ".." {
		return nil, fmt.Errorf("invalid id")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var raw []byte
	err := s.db.QueryRow(ctx, `SELECT archive FROM session_archives WHERE id = $1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}

	var arch SessionArchive
	if err := json.Unmarshal(raw, &arch); err != nil {
		return nil, err
	}
	return &arch, nil
}

func formatTR(t time.Time) string {
	return t.Local().Format("02.01.2006 15:04")
}

func humanDuration(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	total := int(math.Round(sec))
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%dsa %ddk %dsn", h, m, s)
	case m > 0:
		return fmt.Sprintf("%ddk %dsn", m, s)
	default:
		return fmt.Sprintf("%dsn", s)
	}
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
