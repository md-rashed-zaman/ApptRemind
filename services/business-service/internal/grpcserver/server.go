//go:build protogen

package grpcserver

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/config"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	businessv1 "github.com/md-rashed-zaman/apptremind/protos/gen/business/v1"
	"github.com/md-rashed-zaman/apptremind/services/business-service/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	businessv1.UnimplementedBusinessServiceServer
	pool *db.Pool
	repo *storage.Repository
}

func Register(grpcServer *grpc.Server, pool *db.Pool, repo *storage.Repository) {
	businessv1.RegisterBusinessServiceServer(grpcServer, &server{pool: pool, repo: repo})
}

func (s *server) GetBusinessProfile(ctx context.Context, req *businessv1.BusinessProfileRequest) (*businessv1.BusinessProfileResponse, error) {
	offsets := parseOffsets(config.String("REMINDER_OFFSETS_MINUTES", "1440,60"))
	timezone := config.String("TIMEZONE", "UTC")
	name := "Demo Business"

	if s.repo != nil && req.GetBusinessId() != "" {
		p, err := s.repo.GetOrCreateProfile(ctx, req.GetBusinessId())
		if err == nil {
			if strings.TrimSpace(p.Timezone) != "" {
				timezone = strings.TrimSpace(p.Timezone)
			}
			if strings.TrimSpace(p.Name) != "" {
				name = strings.TrimSpace(p.Name)
			}
			if len(p.OffsetsMins) > 0 {
				offsets = nil
				for _, v := range p.OffsetsMins {
					if v <= 0 {
						continue
					}
					offsets = append(offsets, int32(v))
				}
				if len(offsets) == 0 {
					offsets = parseOffsets("1440,60")
				}
			}
		}
	}

	return &businessv1.BusinessProfileResponse{
		BusinessId: req.BusinessId,
		Name:       name,
		ReminderPolicy: &businessv1.ReminderPolicy{
			ReminderOffsetsMinutes: offsets,
			Timezone:               timezone,
		},
	}, nil
}

func (s *server) GetAvailabilityConfig(ctx context.Context, req *businessv1.AvailabilityConfigRequest) (*businessv1.AvailabilityConfigResponse, error) {
	if req.GetBusinessId() == "" || req.GetStaffId() == "" || req.GetServiceId() == "" || req.GetDate() == "" {
		return &businessv1.AvailabilityConfigResponse{
			BusinessId: req.GetBusinessId(),
			StaffId:    req.GetStaffId(),
			ServiceId:  req.GetServiceId(),
			Timezone:   "UTC",
			IsWorking:  false,
		}, nil
	}

	// Defaults if data is missing.
	resp := &businessv1.AvailabilityConfigResponse{
		BusinessId:      req.GetBusinessId(),
		StaffId:         req.GetStaffId(),
		ServiceId:       req.GetServiceId(),
		Timezone:        "UTC",
		DurationMinutes: 30,
		SlotStepMinutes: 15,
		IsWorking:       true,
		WorkStartUtc:    nil,
		WorkEndUtc:      nil,
	}

	if s.repo == nil {
		resp.IsWorking = false
		return resp, nil
	}

	profile, err := s.repo.GetOrCreateProfile(ctx, req.GetBusinessId())
	if err == nil && strings.TrimSpace(profile.Timezone) != "" {
		resp.Timezone = strings.TrimSpace(profile.Timezone)
	}

	durationMins, err := s.repo.GetServiceDuration(ctx, req.GetBusinessId(), req.GetServiceId())
	if err == nil && durationMins > 0 {
		resp.DurationMinutes = int32(durationMins)
	}

	loc, err := time.LoadLocation(resp.Timezone)
	if err != nil {
		loc = time.UTC
		resp.Timezone = "UTC"
	}

	dayLocal, err := time.ParseInLocation("2006-01-02", req.GetDate(), loc)
	if err != nil {
		resp.IsWorking = false
		return resp, nil
	}

	weekday := int(dayLocal.Weekday())
	wh, err := s.repo.GetWorkingHours(ctx, req.GetBusinessId(), req.GetStaffId(), weekday)
	if err != nil {
		resp.IsWorking = false
		return resp, nil
	}
	resp.IsWorking = wh.IsWorking
	if !wh.IsWorking {
		return resp, nil
	}

	startLocal := time.Date(dayLocal.Year(), dayLocal.Month(), dayLocal.Day(), 0, 0, 0, 0, loc).Add(time.Duration(wh.StartMinute) * time.Minute)
	endLocal := time.Date(dayLocal.Year(), dayLocal.Month(), dayLocal.Day(), 0, 0, 0, 0, loc).Add(time.Duration(wh.EndMinute) * time.Minute)
	if !endLocal.After(startLocal) {
		resp.IsWorking = false
		return resp, nil
	}

	workStartUTC := startLocal.UTC()
	workEndUTC := endLocal.UTC()

	// Apply staff time-off blocks (stored as UTC) to produce 0..N availability windows.
	blocks, err := s.repo.ListTimeOff(ctx, req.GetBusinessId(), req.GetStaffId(), workStartUTC, workEndUTC, 500)
	if err != nil {
		// If time-off read fails, fall back to raw working hours.
		resp.WorkStartUtc = timestamppb.New(workStartUTC)
		resp.WorkEndUtc = timestamppb.New(workEndUTC)
		return resp, nil
	}

	windows := subtractBlocks(workStartUTC, workEndUTC, blocks)
	// Back-compat fields: still set a single window that covers the full working hours.
	resp.WorkStartUtc = timestamppb.New(workStartUTC)
	resp.WorkEndUtc = timestamppb.New(workEndUTC)
	for _, w := range windows {
		resp.WindowsUtc = append(resp.WindowsUtc, &businessv1.AvailabilityWindow{
			StartUtc: timestamppb.New(w.Start),
			EndUtc:   timestamppb.New(w.End),
		})
	}
	return resp, nil
}

type interval struct {
	Start time.Time
	End   time.Time
}

func subtractBlocks(baseStart, baseEnd time.Time, blocks []storage.TimeOff) []interval {
	if !baseEnd.After(baseStart) {
		return nil
	}
	var b []interval
	for _, t := range blocks {
		// Clip to base interval.
		s := t.StartTime.UTC()
		e := t.EndTime.UTC()
		if e.Before(baseStart) || !s.Before(baseEnd) {
			continue
		}
		if s.Before(baseStart) {
			s = baseStart
		}
		if e.After(baseEnd) {
			e = baseEnd
		}
		if e.After(s) {
			b = append(b, interval{Start: s, End: e})
		}
	}
	if len(b) == 0 {
		return []interval{{Start: baseStart, End: baseEnd}}
	}

	// Sort and merge overlapping blocks.
	sortIntervals(b)
	merged := make([]interval, 0, len(b))
	for _, cur := range b {
		if len(merged) == 0 {
			merged = append(merged, cur)
			continue
		}
		last := &merged[len(merged)-1]
		if cur.Start.After(last.End) {
			merged = append(merged, cur)
			continue
		}
		if cur.End.After(last.End) {
			last.End = cur.End
		}
	}

	// Subtract merged blocks from base.
	var out []interval
	cursor := baseStart
	for _, m := range merged {
		if m.Start.After(cursor) {
			out = append(out, interval{Start: cursor, End: m.Start})
		}
		if m.End.After(cursor) {
			cursor = m.End
		}
	}
	if baseEnd.After(cursor) {
		out = append(out, interval{Start: cursor, End: baseEnd})
	}
	return out
}

func sortIntervals(in []interval) {
	// Small n; simple insertion sort keeps deps minimal.
	for i := 1; i < len(in); i++ {
		j := i
		for j > 0 && (in[j].Start.Before(in[j-1].Start) || (in[j].Start.Equal(in[j-1].Start) && in[j].End.Before(in[j-1].End))) {
			in[j], in[j-1] = in[j-1], in[j]
			j--
		}
	}
}

func parseOffsets(raw string) []int32 {
	var out []int32
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		mins, err := strconv.Atoi(part)
		if err != nil || mins <= 0 {
			continue
		}
		out = append(out, int32(mins))
	}
	if len(out) == 0 {
		out = []int32{1440}
	}
	return out
}
