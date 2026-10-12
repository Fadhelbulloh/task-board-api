package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/dto"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
	"github.com/Fadhelbulloh/task-board-api/internal/model"
	"github.com/Fadhelbulloh/task-board-api/internal/repository"
)

type TeamService struct {
	tx    Transactor
	teams TeamRepository
	users UserRepository
}

func NewTeamService(tx Transactor, teams TeamRepository, users UserRepository) *TeamService {
	return &TeamService{tx: tx, teams: teams, users: users}
}

// Create inserts the team and the creator's owner membership in one
// transaction. A team without its owner row would be invisible to everyone.
func (s *TeamService) Create(ctx context.Context, userID uuid.UUID, req dto.TeamRequest) (dto.TeamResponse, error) {
	team := model.Team{Name: strings.TrimSpace(req.Name), OwnerID: userID}

	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.teams.Create(ctx, &team); err != nil {
			return err
		}
		return s.teams.AddMember(ctx, model.TeamMember{TeamID: team.ID, UserID: userID, Role: model.TeamRoleOwner})
	})
	if err != nil {
		return dto.TeamResponse{}, helper.ErrInternal(err)
	}
	return dto.NewTeamResponse(team), nil
}

func (s *TeamService) List(ctx context.Context, userID uuid.UUID) ([]dto.TeamResponse, error) {
	teams, err := s.teams.ListByMember(ctx, userID)
	if err != nil {
		return nil, helper.ErrInternal(err)
	}
	res := make([]dto.TeamResponse, len(teams))
	for i, t := range teams {
		res[i] = dto.NewTeamResponse(t)
	}
	return res, nil
}

func (s *TeamService) Get(ctx context.Context, userID, teamID uuid.UUID) (dto.TeamResponse, error) {
	if err := s.authorize(ctx, teamID, userID, false); err != nil {
		return dto.TeamResponse{}, err
	}
	team, err := s.teams.FindByID(ctx, teamID)
	if err != nil {
		return dto.TeamResponse{}, notFoundOr500(err, "team")
	}
	return dto.NewTeamResponse(team), nil
}

func (s *TeamService) Update(ctx context.Context, userID, teamID uuid.UUID, req dto.TeamRequest) (dto.TeamResponse, error) {
	if err := s.authorize(ctx, teamID, userID, true); err != nil {
		return dto.TeamResponse{}, err
	}
	team := model.Team{ID: teamID, Name: strings.TrimSpace(req.Name)}
	if err := s.teams.Update(ctx, &team); err != nil {
		return dto.TeamResponse{}, notFoundOr500(err, "team")
	}
	return dto.NewTeamResponse(team), nil
}

func (s *TeamService) Delete(ctx context.Context, userID, teamID uuid.UUID) error {
	if err := s.authorize(ctx, teamID, userID, true); err != nil {
		return err
	}
	if err := s.teams.Delete(ctx, teamID); err != nil {
		return notFoundOr500(err, "team")
	}
	return nil
}

func (s *TeamService) ListMembers(ctx context.Context, userID, teamID uuid.UUID) ([]dto.TeamMemberResponse, error) {
	if err := s.authorize(ctx, teamID, userID, false); err != nil {
		return nil, err
	}
	members, err := s.teams.ListMembers(ctx, teamID)
	if err != nil {
		return nil, helper.ErrInternal(err)
	}
	res := make([]dto.TeamMemberResponse, len(members))
	for i, m := range members {
		res[i] = dto.NewTeamMemberResponse(m)
	}
	return res, nil
}

// AddMember takes an email rather than a user id so the owner doesn't need
// to know other users' internal ids.
func (s *TeamService) AddMember(ctx context.Context, userID, teamID uuid.UUID, req dto.AddMemberRequest) error {
	if err := s.authorize(ctx, teamID, userID, true); err != nil {
		return err
	}
	user, err := s.users.FindByEmail(ctx, normalizeEmail(req.Email))
	if err != nil {
		return notFoundOr500(err, "user")
	}
	err = s.teams.AddMember(ctx, model.TeamMember{TeamID: teamID, UserID: user.ID, Role: model.TeamRoleMember})
	if errors.Is(err, repository.ErrDuplicate) {
		return helper.ErrConflict("ALREADY_MEMBER", "user is already a member of this team")
	}
	if err != nil {
		return helper.ErrInternal(err)
	}
	return nil
}

func (s *TeamService) RemoveMember(ctx context.Context, userID, teamID, memberID uuid.UUID) error {
	if err := s.authorize(ctx, teamID, userID, true); err != nil {
		return err
	}
	if memberID == userID {
		return helper.ErrValidation("owner cannot leave the team, delete it instead")
	}
	if err := s.teams.RemoveMember(ctx, teamID, memberID); err != nil {
		return notFoundOr500(err, "member")
	}
	return nil
}

// authorize checks the caller belongs to the team, and is its owner when
// ownerOnly is set. Non-members get 404 so team ids can't be probed.
func (s *TeamService) authorize(ctx context.Context, teamID, userID uuid.UUID, ownerOnly bool) error {
	member, err := s.teams.FindMember(ctx, teamID, userID)
	if err != nil {
		return notFoundOr500(err, "team")
	}
	if ownerOnly && member.Role != model.TeamRoleOwner {
		return helper.ErrForbidden("only the team owner can do this")
	}
	return nil
}

func notFoundOr500(err error, resource string) error {
	if errors.Is(err, repository.ErrNotFound) {
		return helper.ErrNotFound(resource)
	}
	return helper.ErrInternal(err)
}
