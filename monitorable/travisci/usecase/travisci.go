//+build !faker

package usecase

import (
	"fmt"
	"time"

	"github.com/monitoror/monitoror/pkg/monitoror/utils/git"

	. "github.com/monitoror/monitoror/models"
	"github.com/monitoror/monitoror/monitorable/travisci"
	"github.com/monitoror/monitoror/monitorable/travisci/models"
	"github.com/monitoror/monitoror/pkg/monitoror/cache"

	. "github.com/AlekSi/pointer"
)

type (
	travisCIUsecase struct {
		repository travisci.Repository

		// builds cache
		buildsCache *cache.BuildCache
	}
)

const cacheSize = 5

func NewTravisCIUsecase(repository travisci.Repository) travisci.Usecase {
	return &travisCIUsecase{repository, cache.NewBuildCache(cacheSize)}
}

func (tu *travisCIUsecase) Build(params *models.BuildParams) (tile *Tile, err error) {
	tile = NewTile(travisci.TravisCIBuildTileType)
	tile.Label = fmt.Sprintf("%s", params.Repository)
	tile.Message = fmt.Sprintf("%s", git.HumanizeBranch(params.Branch))

	// Request
	build, err := tu.repository.GetLastBuildStatus(params.Group, params.Repository, params.Branch)
	if err != nil {
		return nil, &MonitororError{Err: err, Tile: tile, Message: "unable to found build"}
	}
	if build == nil {
		// Warning because request was correct but there is no build
		return nil, &MonitororError{Tile: tile, Message: "unable to found build", ErrorStatus: WarningStatus}
	}

	// Set Status
	tile.Status = parseState(build.State)

	// Set Previous Status
	previousStatus := tu.buildsCache.GetPreviousStatus(tile.Label, fmt.Sprintf("%d", build.Id))
	if previousStatus != nil {
		tile.PreviousStatus = *previousStatus
	} else {
		tile.PreviousStatus = UnknownStatus
	}

	// Set StartedAt
	if !build.StartedAt.IsZero() {
		tile.StartedAt = ToTime(build.StartedAt)
	}
	// Set FinishedAt
	if !build.FinishedAt.IsZero() {
		tile.FinishedAt = ToTime(build.FinishedAt)
	}

	if tile.Status == RunningStatus {
		tile.Duration = ToInt64(int64(time.Now().Sub(build.StartedAt).Seconds()))

		estimatedDuration := tu.buildsCache.GetEstimatedDuration(tile.Label)
		if estimatedDuration != nil {
			tile.EstimatedDuration = ToInt64(int64(estimatedDuration.Seconds()))
		} else {
			tile.EstimatedDuration = ToInt64(int64(0))
		}
	}

	// Set Author
	if build.Author.Name != "" || build.Author.AvatarUrl != "" {
		tile.Author = &Author{
			Name:      build.Author.Name,
			AvatarUrl: build.Author.AvatarUrl,
		}
	}

	// Cache Duration when success / failed
	if tile.Status == SuccessStatus || tile.Status == FailedStatus {
		tu.buildsCache.Add(tile.Label, fmt.Sprintf("%d", build.Id), tile.Status, build.Duration)
	}

	return
}

func parseState(state string) TileStatus {
	switch state {
	case "created":
		return QueuedStatus
	case "received":
		return QueuedStatus
	case "started":
		return RunningStatus
	case "passed":
		return SuccessStatus
	case "failed":
		return FailedStatus
	case "errored":
		return FailedStatus
	case "canceled":
		return AbortedStatus
	default:
		return UnknownStatus
	}
}
