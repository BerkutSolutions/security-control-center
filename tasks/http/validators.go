package taskshttp

import (
	"errors"
	"strings"
	"time"

	"berkut-scc/tasks"
)

func normalizePriority(val string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(val))
	if p == "" {
		p = tasks.PriorityMedium
	}
	if !isValidPriority(p) {
		return "", errors.New("tasks.priorityInvalid")
	}
	return p, nil
}

func validatePriority(val string) error {
	p := strings.ToLower(strings.TrimSpace(val))
	if !isValidPriority(p) {
		return errors.New("tasks.priorityInvalid")
	}
	return nil
}

func isValidPriority(val string) bool {
	switch strings.ToLower(val) {
	case tasks.PriorityLow, tasks.PriorityMedium, tasks.PriorityHigh, tasks.PriorityCritical:
		return true
	default:
		return false
	}
}

func parseDueDate(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	if strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	parsed, err := parseISOTime(*raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseISOTime(val string) (time.Time, error) {
	clean := strings.TrimSpace(val)
	if clean == "" {
		return time.Time{}, errors.New("empty time")
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"}
	var lastErr error
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, clean); err == nil {
			return ts.UTC(), nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}

func normalizeChecklist(items []tasks.TaskChecklistItem, existing []tasks.TaskChecklistItem, userID int64, now time.Time) []tasks.TaskChecklistItem {
	var res []tasks.TaskChecklistItem
	for i, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		item.Text = text
		var prev *tasks.TaskChecklistItem
		if i < len(existing) && strings.TrimSpace(existing[i].Text) == text {
			prev = &existing[i]
		}
		if item.Done {
			if prev != nil && prev.Done && prev.DoneBy != nil && prev.DoneAt != nil {
				item.DoneBy = prev.DoneBy
				item.DoneAt = prev.DoneAt
			} else {
				item.DoneBy = &userID
				item.DoneAt = &now
			}
		} else {
			item.DoneBy = nil
			item.DoneAt = nil
		}
		res = append(res, item)
	}
	return res
}
