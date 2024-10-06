package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

type ReserveLivestreamRequest struct {
	Tags         []int64 `json:"tags"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	PlaylistUrl  string  `json:"playlist_url"`
	ThumbnailUrl string  `json:"thumbnail_url"`
	StartAt      int64   `json:"start_at"`
	EndAt        int64   `json:"end_at"`
}

type LivestreamViewerModel struct {
	UserID       int64 `db:"user_id" json:"user_id"`
	LivestreamID int64 `db:"livestream_id" json:"livestream_id"`
	CreatedAt    int64 `db:"created_at" json:"created_at"`
}

type LivestreamModel struct {
	ID           int64  `db:"id" json:"id"`
	UserID       int64  `db:"user_id" json:"user_id"`
	Title        string `db:"title" json:"title"`
	Description  string `db:"description" json:"description"`
	PlaylistUrl  string `db:"playlist_url" json:"playlist_url"`
	ThumbnailUrl string `db:"thumbnail_url" json:"thumbnail_url"`
	StartAt      int64  `db:"start_at" json:"start_at"`
	EndAt        int64  `db:"end_at" json:"end_at"`
}

type LivestreamModel2 struct {
	// livestreams
	Livestream_ID           int64  `db:"livestream_id"`
	Livestream_Title        string `db:"livestream_title"`
	Livestream_Description  string `db:"livestream_description"`
	Livestream_PlaylistUrl  string `db:"livestream_playlist_url"`
	Livestream_ThumbnailUrl string `db:"livestream_thumbnail_url"`
	Livestream_StartAt      int64  `db:"livestream_start_at"`
	Livestream_EndAt        int64  `db:"livestream_end_at"`
	// livestream_owners
	LivestreamOwner_ID          int64  `db:"livestream_owner_id"`
	LivestreamOwner_Name        string `db:"livestream_owner_name"`
	LivestreamOwner_DisplayName string `db:"livestream_owner_display_name"`
	LivestreamOwner_Description string `db:"livestream_owner_description"`
	// livestream_owner_themes
	LivestreamOwnerTheme_ID       int64 `db:"livestream_owner_theme_id"`
	LivestreamOwnerTheme_DarkMode bool  `db:"livestream_owner_theme_dark_mode"`
}

type Livestream struct {
	ID           int64  `json:"id"`
	Owner        User   `json:"owner"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	PlaylistUrl  string `json:"playlist_url"`
	ThumbnailUrl string `json:"thumbnail_url"`
	Tags         []Tag  `json:"tags"`
	StartAt      int64  `json:"start_at"`
	EndAt        int64  `json:"end_at"`
}

type LivestreamTagModel struct {
	ID           int64 `db:"id" json:"id"`
	LivestreamID int64 `db:"livestream_id" json:"livestream_id"`
	TagID        int64 `db:"tag_id" json:"tag_id"`
}

type LivestreamTagModel2 struct {
	LivestreamID int64 `db:"livestream_id"`
	TagID        int64 `db:"tag_id"`
}

type ReservationSlotModel struct {
	ID      int64 `db:"id" json:"id"`
	Slot    int64 `db:"slot" json:"slot"`
	StartAt int64 `db:"start_at" json:"start_at"`
	EndAt   int64 `db:"end_at" json:"end_at"`
}

func reserveLivestreamHandler(c echo.Context) error {
	ctx := c.Request().Context()
	defer c.Request().Body.Close()

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	var req *ReserveLivestreamRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to decode the request body as json")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	// 2023/11/25 10:00からの１年間の期間内であるかチェック
	var (
		termStartAt    = time.Date(2023, 11, 25, 1, 0, 0, 0, time.UTC)
		termEndAt      = time.Date(2024, 11, 25, 1, 0, 0, 0, time.UTC)
		reserveStartAt = time.Unix(req.StartAt, 0)
		reserveEndAt   = time.Unix(req.EndAt, 0)
	)
	if (reserveStartAt.Equal(termEndAt) || reserveStartAt.After(termEndAt)) || (reserveEndAt.Equal(termStartAt) || reserveEndAt.Before(termStartAt)) {
		return echo.NewHTTPError(http.StatusBadRequest, "bad reservation time range")
	}

	// 予約枠をみて、予約が可能か調べる
	// NOTE: 並列な予約のoverbooking防止にFOR UPDATEが必要
	type SlotCount struct {
		Count int64 `db:"cnt"`
	}
	slotCount := SlotCount{}
	if err := tx.GetContext(ctx, &slotCount, "SELECT count(1) as cnt FROM reservation_slots WHERE start_at >= ? AND end_at <= ? AND slot = 0 FOR UPDATE", req.StartAt, req.EndAt); err != nil {
		c.Logger().Warnf("予約枠一覧取得でエラー発生: %+v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get reservation_slots: "+err.Error())
	}
	if 0 < slotCount.Count {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("予約期間 %d ~ %dに対して、予約区間 %d ~ %dが予約できません", termStartAt.Unix(), termEndAt.Unix(), req.StartAt, req.EndAt))
	}

	var (
		livestreamModel = &LivestreamModel{
			UserID:       int64(userID),
			Title:        req.Title,
			Description:  req.Description,
			PlaylistUrl:  req.PlaylistUrl,
			ThumbnailUrl: req.ThumbnailUrl,
			StartAt:      req.StartAt,
			EndAt:        req.EndAt,
		}
	)

	if _, err := tx.ExecContext(ctx, "UPDATE reservation_slots SET slot = slot - 1 WHERE start_at >= ? AND end_at <= ?", req.StartAt, req.EndAt); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update reservation_slot: "+err.Error())
	}
	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	rs, err := dbConn.NamedExecContext(ctx, "INSERT INTO livestreams (user_id, title, description, playlist_url, thumbnail_url, start_at, end_at) VALUES(:user_id, :title, :description, :playlist_url, :thumbnail_url, :start_at, :end_at)", livestreamModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert livestream: "+err.Error())
	}

	livestreamID, err := rs.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get last inserted livestream id: "+err.Error())
	}
	livestreamModel.ID = livestreamID

	// タグ追加
	ptrTags := make([]*Tag, len(req.Tags))
	tags := make([]Tag, len(req.Tags))
	insertTags := make([]LivestreamTagModel2, len(req.Tags))
	for i, tagID := range req.Tags {
		ptrTags[i] = getPtrTagByID(tagID)
		tags[i] = *(ptrTags[i])
		insertTags[i] = LivestreamTagModel2{
			LivestreamID: livestreamID,
			TagID:        tagID,
		}
	}
	if 0 < len(insertTags) {
		if _, err := dbConn.NamedExecContext(ctx, "INSERT INTO livestream_tags (livestream_id, tag_id) VALUES (:livestream_id, :tag_id)", insertTags); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert livestream tag: "+err.Error())
		}
	}
	livestreamTags.Set(strconv.FormatInt(livestreamID, 10), ptrTags)

	query := `
select
  livestreams.id as "livestream_id"
  , livestreams.title as "livestream_title"
  , livestreams.description as "livestream_description"
  , livestreams.playlist_url as "livestream_playlist_url"
  , livestreams.thumbnail_url as "livestream_thumbnail_url"
  , livestreams.start_at as "livestream_start_at"
  , livestreams.end_at as "livestream_end_at"
  , livestream_owners.id as "livestream_owner_id"
  , livestream_owners.name as "livestream_owner_name"
  , livestream_owners.display_name as "livestream_owner_display_name"
  , livestream_owners.description as "livestream_owner_description"
  , livestream_owner_themes.id as "livestream_owner_theme_id"
  , livestream_owner_themes.dark_mode as "livestream_owner_theme_dark_mode"
from livestreams
inner join users as livestream_owners on livestream_owners.id = livestreams.user_id
inner join themes as livestream_owner_themes on livestream_owner_themes.user_id = livestream_owners.id
where livestreams.id = ?
`
	livestreamModel2 := LivestreamModel2{}
	err = dbConn.GetContext(ctx, &livestreamModel2, query, livestreamID)
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "not found livestream that has the given id")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream: "+err.Error())
	}

	livestream := Livestream{
		ID: livestreamModel2.Livestream_ID,
		Owner: User{
			ID:          livestreamModel2.LivestreamOwner_ID,
			Name:        livestreamModel2.LivestreamOwner_Name,
			DisplayName: livestreamModel2.LivestreamOwner_DisplayName,
			Description: livestreamModel2.LivestreamOwner_Description,
			Theme: Theme{
				ID:       livestreamModel2.LivestreamOwnerTheme_ID,
				DarkMode: livestreamModel2.LivestreamOwnerTheme_DarkMode,
			},
			IconHash: getIconHashByUserId(livestreamModel2.LivestreamOwner_ID),
		},
		Title:        livestreamModel2.Livestream_Title,
		Description:  livestreamModel2.Livestream_Description,
		PlaylistUrl:  livestreamModel2.Livestream_PlaylistUrl,
		ThumbnailUrl: livestreamModel2.Livestream_ThumbnailUrl,
		Tags:         tags,
		StartAt:      livestreamModel2.Livestream_StartAt,
		EndAt:        livestreamModel2.Livestream_EndAt,
	}
	return c.JSON(http.StatusCreated, livestream)
}

func searchLivestreamsHandler(c echo.Context) error {
	ctx := c.Request().Context()
	keyTagName := c.QueryParam("tag")

	var livestreamModels []*LivestreamModel2
	if c.QueryParam("tag") != "" {
		query := `
with livestream_ids as (
select livestream_tags.livestream_id
from tags
inner join livestream_tags on tags.id = livestream_tags.tag_id
where tags.name = ?
)
select
  livestreams.id as "livestream_id"
  , livestreams.title as "livestream_title"
  , livestreams.description as "livestream_description"
  , livestreams.playlist_url as "livestream_playlist_url"
  , livestreams.thumbnail_url as "livestream_thumbnail_url"
  , livestreams.start_at as "livestream_start_at"
  , livestreams.end_at as "livestream_end_at"
  , livestream_owners.id as "livestream_owner_id"
  , livestream_owners.name as "livestream_owner_name"
  , livestream_owners.display_name as "livestream_owner_display_name"
  , livestream_owners.description as "livestream_owner_description"
  , livestream_owner_themes.id as "livestream_owner_theme_id"
  , livestream_owner_themes.dark_mode as "livestream_owner_theme_dark_mode"
from livestreams
inner join users as livestream_owners on livestream_owners.id = livestreams.user_id
inner join themes as livestream_owner_themes on livestream_owner_themes.user_id = livestream_owners.id
where livestreams.id in (select livestream_ids.livestream_id from livestream_ids)
order by livestream_id desc
;`
		if err := dbConn.SelectContext(ctx, &livestreamModels, query, keyTagName); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestreams: "+err.Error())
		}
	} else {
		// kaizen-04: 1発で取得
		// // 検索条件なし
		// query := `SELECT * FROM livestreams ORDER BY id DESC`
		query := `
select
  livestreams.id as "livestream_id"
  , livestreams.title as "livestream_title"
  , livestreams.description as "livestream_description"
  , livestreams.playlist_url as "livestream_playlist_url"
  , livestreams.thumbnail_url as "livestream_thumbnail_url"
  , livestreams.start_at as "livestream_start_at"
  , livestreams.end_at as "livestream_end_at"
  , livestream_owners.id as "livestream_owner_id"
  , livestream_owners.name as "livestream_owner_name"
  , livestream_owners.display_name as "livestream_owner_display_name"
  , livestream_owners.description as "livestream_owner_description"
  , livestream_owner_themes.id as "livestream_owner_theme_id"
  , livestream_owner_themes.dark_mode as "livestream_owner_theme_dark_mode"
from livestreams
inner join users as livestream_owners on livestream_owners.id = livestreams.user_id
inner join themes as livestream_owner_themes on livestream_owner_themes.user_id = livestream_owners.id
order by livestream_id desc
`
		if c.QueryParam("limit") != "" {
			limit, err := strconv.Atoi(c.QueryParam("limit"))
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "limit query parameter must be integer")
			}
			query += fmt.Sprintf(" LIMIT %d", limit)
		}

		if err := dbConn.SelectContext(ctx, &livestreamModels, query); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestreams: "+err.Error())
		}
	}

	livestreams := make([]Livestream, len(livestreamModels))
	for i := range livestreamModels {
		var tags []Tag
		tags, err := getLivestreamTags2(ctx, livestreamModels[i].Livestream_ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream tags: "+err.Error())
		}
		livestream := Livestream{
			ID: livestreamModels[i].Livestream_ID,
			Owner: User{
				ID:          livestreamModels[i].LivestreamOwner_ID,
				Name:        livestreamModels[i].LivestreamOwner_Name,
				DisplayName: livestreamModels[i].LivestreamOwner_DisplayName,
				Description: livestreamModels[i].LivestreamOwner_Description,
				Theme: Theme{
					ID:       livestreamModels[i].LivestreamOwnerTheme_ID,
					DarkMode: livestreamModels[i].LivestreamOwnerTheme_DarkMode,
				},
				IconHash: getIconHashByUserId(livestreamModels[i].LivestreamOwner_ID),
			},
			Title:        livestreamModels[i].Livestream_Title,
			Description:  livestreamModels[i].Livestream_Description,
			PlaylistUrl:  livestreamModels[i].Livestream_PlaylistUrl,
			ThumbnailUrl: livestreamModels[i].Livestream_ThumbnailUrl,
			Tags:         tags,
			StartAt:      livestreamModels[i].Livestream_StartAt,
			EndAt:        livestreamModels[i].Livestream_EndAt,
		}
		livestreams[i] = livestream
	}

	return c.JSON(http.StatusOK, livestreams)
}

func getMyLivestreamsHandler(c echo.Context) error {
	ctx := c.Request().Context()
	if err := verifyUserSession(c); err != nil {
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	query := `
select
  livestreams.id as "livestream_id"
  , livestreams.title as "livestream_title"
  , livestreams.description as "livestream_description"
  , livestreams.playlist_url as "livestream_playlist_url"
  , livestreams.thumbnail_url as "livestream_thumbnail_url"
  , livestreams.start_at as "livestream_start_at"
  , livestreams.end_at as "livestream_end_at"
  , livestream_owners.id as "livestream_owner_id"
  , livestream_owners.name as "livestream_owner_name"
  , livestream_owners.display_name as "livestream_owner_display_name"
  , livestream_owners.description as "livestream_owner_description"
  , livestream_owner_themes.id as "livestream_owner_theme_id"
  , livestream_owner_themes.dark_mode as "livestream_owner_theme_dark_mode"
from livestreams
inner join users as livestream_owners on livestream_owners.id = livestreams.user_id
inner join themes as livestream_owner_themes on livestream_owner_themes.user_id = livestream_owners.id
where livestream_owners.id = ?
`
	livestreamModels := []LivestreamModel2{}
	err := dbConn.SelectContext(ctx, &livestreamModels, query, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "not found livestream that has the given id")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream: "+err.Error())
	}

	livestreams := make([]Livestream, len(livestreamModels))
	for i, livestreamModel := range livestreamModels {
		tags, err := getLivestreamTags2(ctx, livestreamModel.Livestream_ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream tags: "+err.Error())
		}
		livestream := Livestream{
			ID: livestreamModel.Livestream_ID,
			Owner: User{
				ID:          livestreamModel.LivestreamOwner_ID,
				Name:        livestreamModel.LivestreamOwner_Name,
				DisplayName: livestreamModel.LivestreamOwner_DisplayName,
				Description: livestreamModel.LivestreamOwner_Description,
				Theme: Theme{
					ID:       livestreamModel.LivestreamOwnerTheme_ID,
					DarkMode: livestreamModel.LivestreamOwnerTheme_DarkMode,
				},
				IconHash: getIconHashByUserId(livestreamModel.LivestreamOwner_ID),
			},
			Title:        livestreamModel.Livestream_Title,
			Description:  livestreamModel.Livestream_Description,
			PlaylistUrl:  livestreamModel.Livestream_PlaylistUrl,
			ThumbnailUrl: livestreamModel.Livestream_ThumbnailUrl,
			Tags:         tags,
			StartAt:      livestreamModel.Livestream_StartAt,
			EndAt:        livestreamModel.Livestream_EndAt,
		}
		livestreams[i] = livestream
	}

	return c.JSON(http.StatusOK, livestreams)
}

func getUserLivestreamsHandler(c echo.Context) error {
	ctx := c.Request().Context()
	if err := verifyUserSession(c); err != nil {
		return err
	}

	username := c.Param("username")

	query := `
select
  livestreams.id as "livestream_id"
  , livestreams.title as "livestream_title"
  , livestreams.description as "livestream_description"
  , livestreams.playlist_url as "livestream_playlist_url"
  , livestreams.thumbnail_url as "livestream_thumbnail_url"
  , livestreams.start_at as "livestream_start_at"
  , livestreams.end_at as "livestream_end_at"
  , livestream_owners.id as "livestream_owner_id"
  , livestream_owners.name as "livestream_owner_name"
  , livestream_owners.display_name as "livestream_owner_display_name"
  , livestream_owners.description as "livestream_owner_description"
  , livestream_owner_themes.id as "livestream_owner_theme_id"
  , livestream_owner_themes.dark_mode as "livestream_owner_theme_dark_mode"
from livestreams
inner join users as livestream_owners on livestream_owners.id = livestreams.user_id
inner join themes as livestream_owner_themes on livestream_owner_themes.user_id = livestream_owners.id
where livestream_owners.name = ?
`
	livestreamModels := []LivestreamModel2{}
	err := dbConn.SelectContext(ctx, &livestreamModels, query, username)
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "not found livestream that has the given id")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream: "+err.Error())
	}

	livestreams := make([]Livestream, len(livestreamModels))
	for i, livestreamModel := range livestreamModels {
		tags, err := getLivestreamTags2(ctx, livestreamModel.Livestream_ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream tags: "+err.Error())
		}
		livestream := Livestream{
			ID: livestreamModel.Livestream_ID,
			Owner: User{
				ID:          livestreamModel.LivestreamOwner_ID,
				Name:        livestreamModel.LivestreamOwner_Name,
				DisplayName: livestreamModel.LivestreamOwner_DisplayName,
				Description: livestreamModel.LivestreamOwner_Description,
				Theme: Theme{
					ID:       livestreamModel.LivestreamOwnerTheme_ID,
					DarkMode: livestreamModel.LivestreamOwnerTheme_DarkMode,
				},
				IconHash: getIconHashByUserId(livestreamModel.LivestreamOwner_ID),
			},
			Title:        livestreamModel.Livestream_Title,
			Description:  livestreamModel.Livestream_Description,
			PlaylistUrl:  livestreamModel.Livestream_PlaylistUrl,
			ThumbnailUrl: livestreamModel.Livestream_ThumbnailUrl,
			Tags:         tags,
			StartAt:      livestreamModel.Livestream_StartAt,
			EndAt:        livestreamModel.Livestream_EndAt,
		}
		livestreams[i] = livestream
	}

	return c.JSON(http.StatusOK, livestreams)
}

// viewerテーブルの廃止
func enterLivestreamHandler(c echo.Context) error {
	ctx := c.Request().Context()
	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id must be integer")
	}

	viewer := LivestreamViewerModel{
		UserID:       int64(userID),
		LivestreamID: int64(livestreamID),
		CreatedAt:    time.Now().Unix(),
	}

	if _, err := dbConn.NamedExecContext(ctx, "INSERT INTO livestream_viewers_history (user_id, livestream_id, created_at) VALUES(:user_id, :livestream_id, :created_at)", viewer); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert livestream_view_history: "+err.Error())
	}

	return c.NoContent(http.StatusOK)
}

func exitLivestreamHandler(c echo.Context) error {
	ctx := c.Request().Context()
	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	if _, err := dbConn.ExecContext(ctx, "DELETE FROM livestream_viewers_history WHERE user_id = ? AND livestream_id = ?", userID, livestreamID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete livestream_view_history: "+err.Error())
	}

	return c.NoContent(http.StatusOK)
}

func getLivestreamHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		return err
	}

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	query := `
select
  livestreams.id as "livestream_id"
  , livestreams.title as "livestream_title"
  , livestreams.description as "livestream_description"
  , livestreams.playlist_url as "livestream_playlist_url"
  , livestreams.thumbnail_url as "livestream_thumbnail_url"
  , livestreams.start_at as "livestream_start_at"
  , livestreams.end_at as "livestream_end_at"
  , livestream_owners.id as "livestream_owner_id"
  , livestream_owners.name as "livestream_owner_name"
  , livestream_owners.display_name as "livestream_owner_display_name"
  , livestream_owners.description as "livestream_owner_description"
  , livestream_owner_themes.id as "livestream_owner_theme_id"
  , livestream_owner_themes.dark_mode as "livestream_owner_theme_dark_mode"
from livestreams
inner join users as livestream_owners on livestream_owners.id = livestreams.user_id
inner join themes as livestream_owner_themes on livestream_owner_themes.user_id = livestream_owners.id
where livestreams.id = ?
`
	livestreamModel := LivestreamModel2{}
	err = dbConn.GetContext(ctx, &livestreamModel, query, livestreamID)
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "not found livestream that has the given id")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream: "+err.Error())
	}
	tags, err := getLivestreamTags2(ctx, livestreamModel.Livestream_ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream tags: "+err.Error())
	}

	livestream := Livestream{
		ID: livestreamModel.Livestream_ID,
		Owner: User{
			ID:          livestreamModel.LivestreamOwner_ID,
			Name:        livestreamModel.LivestreamOwner_Name,
			DisplayName: livestreamModel.LivestreamOwner_DisplayName,
			Description: livestreamModel.LivestreamOwner_Description,
			Theme: Theme{
				ID:       livestreamModel.LivestreamOwnerTheme_ID,
				DarkMode: livestreamModel.LivestreamOwnerTheme_DarkMode,
			},
			IconHash: getIconHashByUserId(livestreamModel.LivestreamOwner_ID),
		},
		Title:        livestreamModel.Livestream_Title,
		Description:  livestreamModel.Livestream_Description,
		PlaylistUrl:  livestreamModel.Livestream_PlaylistUrl,
		ThumbnailUrl: livestreamModel.Livestream_ThumbnailUrl,
		Tags:         tags,
		StartAt:      livestreamModel.Livestream_StartAt,
		EndAt:        livestreamModel.Livestream_EndAt,
	}

	return c.JSON(http.StatusOK, livestream)
}

// 指定した livestream_id に紐づく
// 指定したLivestreamは自分のものであること
func getLivecommentReportsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		return err
	}

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	// error already check
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already check
	userID := sess.Values[defaultUserIDKey].(int64)

	query := `
select
  livecomment_reports.id as "livecomment_report_id"
  , livecomment_reports.created_at as "livecomment_report_created_at"
  , users.id as "user_id"
  , users.name as "user_name"
  , users.display_name as "user_display_name"
  , users.description as "user_description"
  , themes.id as "theme_id"
  , themes.dark_mode as "theme_dark_mode"
  , livecomments.id as "livecomment_id"
  , livecomments.comment as "livecomment_comment"
  , livecomments.tip as "livecomment_tip"
  , livecomments.created_at as "livecomment_created_at"
  , commenters.id as "commenter_id"
  , commenters.name as "commenter_name"
  , commenters.display_name as "commenter_display_name"
  , commenters.description as "commenter_description"
  , commenter_themes.id as "commenter_theme_id"
  , commenter_themes.dark_mode as "commenter_theme_dark_mode"
  , livestreams.id as "livestream_id"
  , livestreams.title as "livestream_title"
  , livestreams.description as "livestream_description"
  , livestreams.playlist_url as "livestream_playlist_url"
  , livestreams.thumbnail_url as "livestream_thumbnail_url"
  , livestreams.start_at as "livestream_start_at"
  , livestreams.end_at as "livestream_end_at"
  , livestream_owners.id as "livestream_owner_id"
  , livestream_owners.name as "livestream_owner_name"
  , livestream_owners.display_name as "livestream_owner_display_name"
  , livestream_owners.description as "livestream_owner_description"
  , livestream_owner_themes.id as "livestream_owner_theme_id"
  , livestream_owner_themes.dark_mode as "livestream_owner_theme_dark_mode"
from livecomment_reports
inner join users on users.id = livecomment_reports.user_id
inner join themes on themes.user_id = users.id
inner join livecomments on livecomment_reports.livecomment_id = livecomments.id
inner join users as commenters on commenters.id = livecomments.user_id
inner join themes as commenter_themes on commenter_themes.user_id = commenters.id
inner join livestreams on livestreams.id = livecomments.livestream_id
inner join users as livestream_owners on livestream_owners.id = livestreams.user_id
inner join themes as livestream_owner_themes on livestream_owner_themes.user_id = livestream_owners.id
where livestreams.id = ? and livestream_owners.id = ?
`
	var reportModels []*LivecommentReportModel2
	if err := dbConn.SelectContext(ctx, &reportModels, query, livestreamID, userID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livecomment reports: "+err.Error())
	}

	reports := make([]LivecommentReport, len(reportModels))
	for i, livecommentReportModel2 := range reportModels {
		tags, err := getLivestreamTags2(ctx, livecommentReportModel2.Livestream_ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream tags: "+err.Error())
		}
		reports[i] = LivecommentReport{
			ID: livecommentReportModel2.LivecommentReport_ID,
			Reporter: User{
				ID:          livecommentReportModel2.User_ID,
				Name:        livecommentReportModel2.User_Name,
				DisplayName: livecommentReportModel2.User_DisplayName,
				Description: livecommentReportModel2.User_Description,
				Theme: Theme{
					ID:       livecommentReportModel2.Theme_ID,
					DarkMode: livecommentReportModel2.Theme_DarkMode,
				},
				IconHash: getIconHashByUserId(livecommentReportModel2.User_ID),
			},
			Livecomment: Livecomment{
				ID: livecommentReportModel2.Livecomment_ID,
				User: User{
					ID:          livecommentReportModel2.Commenter_ID,
					Name:        livecommentReportModel2.Commenter_Name,
					DisplayName: livecommentReportModel2.Commenter_DisplayName,
					Description: livecommentReportModel2.Commenter_Description,
					Theme: Theme{
						ID:       livecommentReportModel2.CommenterTheme_ID,
						DarkMode: livecommentReportModel2.CommenterTheme_DarkMode,
					},
					IconHash: getIconHashByUserId(livecommentReportModel2.Commenter_ID),
				},
				Livestream: Livestream{
					ID:           livecommentReportModel2.Livestream_ID,
					Title:        livecommentReportModel2.Livestream_Title,
					Description:  livecommentReportModel2.Livestream_Description,
					PlaylistUrl:  livecommentReportModel2.Livestream_PlaylistUrl,
					ThumbnailUrl: livecommentReportModel2.Livestream_ThumbnailUrl,
					Tags:         tags,
					StartAt:      livecommentReportModel2.Livestream_StartAt,
					EndAt:        livecommentReportModel2.Livestream_EndAt,
					Owner: User{
						ID:          livecommentReportModel2.LivestreamOwner_ID,
						Name:        livecommentReportModel2.LivestreamOwner_Name,
						DisplayName: livecommentReportModel2.LivestreamOwner_DisplayName,
						Description: livecommentReportModel2.LivestreamOwner_Description,
						Theme: Theme{
							ID:       livecommentReportModel2.LivestreamOwnerTheme_ID,
							DarkMode: livecommentReportModel2.LivestreamOwnerTheme_DarkMode,
						},
						IconHash: getIconHashByUserId(livecommentReportModel2.LivestreamOwner_ID),
					},
				},
				Comment:   livecommentReportModel2.Livecomment_Comment,
				Tip:       livecommentReportModel2.Livecomment_Tip,
				CreatedAt: livecommentReportModel2.Livecomment_CreatedAt,
			},
			CreatedAt: livecommentReportModel2.LivecommentReport_CreatedAt,
		}
	}

	return c.JSON(http.StatusOK, reports)
}
