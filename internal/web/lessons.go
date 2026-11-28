package web

import (
	"encoding/json"
	"hash/fnv"
	"net/http"
	"strconv"

	"github.com/withworkspace/workspace/internal/ops"
)

// The Lessons page: what crew keep running into where they work, read
// from their handoffs (internal/lessons), each with what would stop it;
// one made a mission of, marked fixed or dismissed, or opened again.

// Lessoning is what the Lessons page asks of the workspace.
type Lessoning interface {
	Lessons() (ops.LessonsInfo, error)
	SetLessonState(id int64, state string, by ops.Actor) error
	LessonMission(id int64, by ops.Actor) (string, error)
	// LessonsStamp names where they stand, cheaply: it moves with each
	// write to them.
	LessonsStamp() string
}

// LessonsInfo is what GET /api/lessons serves: every lesson,
// recommendations first, and how reading handoffs for them stands.
type LessonsInfo struct {
	ops.LessonsInfo
}

// LessonState is what POST /api/lessons/{id}/state takes: open, fixed or
// dismissed.
type LessonState struct {
	State string `json:"state"`
}

// LessonMissionMade is what POST /api/lessons/{id}/mission answers: the
// mission made of it (or made before), waiting to be briefed.
type LessonMissionMade struct {
	Mission string `json:"mission"`
}

func lessonID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := ops.LessonOf(r.PathValue("id"))
	if !ok {
		writeError(w, ErrNotFound, http.StatusNotFound)
	}
	return id, ok
}

// lessons is GET /api/lessons.
func (s *Server) lessons(w http.ResponseWriter, r *http.Request) {
	info, err := s.App.Lessons()
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, LessonsInfo{info})
}

// lessonState is POST /api/lessons/{id}/state.
func (s *Server) lessonState(w http.ResponseWriter, r *http.Request) {
	id, ok := lessonID(w, r)
	if !ok {
		return
	}
	var b LessonState
	if !readBody(w, r, &b) {
		return
	}
	if err := s.App.SetLessonState(id, b.State, ops.AsYou); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// lessonMission is POST /api/lessons/{id}/mission.
func (s *Server) lessonMission(w http.ResponseWriter, r *http.Request) {
	id, ok := lessonID(w, r)
	if !ok {
		return
	}
	slug, err := s.App.LessonMission(id, ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, LessonMissionMade{Mission: slug})
}

// lessonsFingerprint is what the Lessons page is built from, hashed: it
// moves when a lesson does, or a handoff's waiting to be read.
func (s *Server) lessonsFingerprint() string {
	info, err := s.App.Lessons()
	if err != nil {
		return "error: " + err.Error()
	}
	data, _ := json.Marshal(info)
	h := fnv.New64a()
	h.Write(data)
	return strconv.FormatUint(h.Sum64(), 16)
}
