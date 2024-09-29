package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

type ReactionModel struct {
	ID           int64  `db:"id"`
	EmojiName    string `db:"emoji_name"`
	UserID       int64  `db:"user_id"`
	LivestreamID int64  `db:"livestream_id"`
	CreatedAt    int64  `db:"created_at"`
}

// kaizen-02: 1発で取得(N+1を解決する)
type ReactionModel2 struct {
	// reactions
	Reaction_ID        int64  `db:"reaction_id"`
	Reaction_EmojiName string `db:"reaction_emoji_name"`
	Reaction_CreatedAt int64  `db:"reaction_created_at"`
	// users
	User_ID          int64  `db:"user_id"`
	User_Name        string `db:"user_name"`
	User_DisplayName string `db:"user_display_name"`
	User_Description string `db:"user_description"`
	// themes
	Theme_ID       int64 `db:"theme_id"`
	Theme_DarkMode bool  `db:"theme_dark_mode"`
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

type Reaction struct {
	ID         int64      `json:"id"`
	EmojiName  string     `json:"emoji_name"`
	User       User       `json:"user"`
	Livestream Livestream `json:"livestream"`
	CreatedAt  int64      `json:"created_at"`
}

type PostReactionRequest struct {
	EmojiName string `json:"emoji_name"`
}

func getReactionsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	// kaizen-02: 1発で取得(N+1を解決する)
	// query := "SELECT * FROM reactions WHERE livestream_id = ? ORDER BY created_at DESC"
	query := `
select
  reactions.id as "reaction_id"
  , reactions.emoji_name as "reaction_emoji_name"
  , reactions.created_at as "reaction_created_at"
  , users.id as "user_id"
  , users.name as "user_name"
  , users.display_name as "user_display_name"
  , users.description as "user_description"
  , themes.id as "theme_id"
  , themes.dark_mode as "theme_dark_mode"
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
from reactions
inner join users on users.id = reactions.user_id
inner join themes on themes.user_id = users.id
inner join livestreams on livestreams.id = reactions.livestream_id
inner join users as livestream_owners on livestream_owners.id = livestreams.user_id
inner join themes as livestream_owner_themes on livestream_owner_themes.user_id = livestream_owners.id
where reactions.livestream_id = ?
order by created_at desc
`
	if c.QueryParam("limit") != "" {
		limit, err := strconv.Atoi(c.QueryParam("limit"))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "limit query parameter must be integer")
		}
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	// kaizen-02: 1発で取得(N+1を解決する)
	//reactionModels := []ReactionModel{}
	reactionModels := []ReactionModel2{}
	if err := dbConn.SelectContext(ctx, &reactionModels, query, livestreamID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "failed to get reactions")
	}

	// kaizen-02: tagsのUnmarshalは1回だけにして、使い回す
	var tags []Tag
	if len(reactionModels) > 0 {
		tags, err = getLivestreamTags2(ctx, reactionModels[0].Livestream_ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
		}
	}

	reactions := make([]Reaction, len(reactionModels))
	for i := range reactionModels {
		reactions[i] = Reaction{
			ID:        reactionModels[i].Reaction_ID,
			EmojiName: reactionModels[i].Reaction_EmojiName,
			User: User{
				ID:          reactionModels[i].User_ID,
				Name:        reactionModels[i].User_Name,
				DisplayName: reactionModels[i].User_DisplayName,
				Description: reactionModels[i].User_Description,
				Theme: Theme{
					ID:       reactionModels[i].Theme_ID,
					DarkMode: reactionModels[i].Theme_DarkMode,
				},
				IconHash: getIconHashByUserId(reactionModels[i].User_ID),
			},
			Livestream: Livestream{
				ID: reactionModels[i].Livestream_ID,
				Owner: User{
					ID:          reactionModels[i].LivestreamOwner_ID,
					Name:        reactionModels[i].LivestreamOwner_Name,
					DisplayName: reactionModels[i].LivestreamOwner_DisplayName,
					Description: reactionModels[i].LivestreamOwner_Description,
					Theme: Theme{
						ID:       reactionModels[i].LivestreamOwnerTheme_ID,
						DarkMode: reactionModels[i].LivestreamOwnerTheme_DarkMode,
					},
					IconHash: getIconHashByUserId(reactionModels[i].LivestreamOwner_ID),
				},
				Title:        reactionModels[i].Livestream_Title,
				Description:  reactionModels[i].Livestream_Description,
				PlaylistUrl:  reactionModels[i].Livestream_PlaylistUrl,
				ThumbnailUrl: reactionModels[i].Livestream_ThumbnailUrl,
				Tags:         tags,
				StartAt:      reactionModels[i].Livestream_StartAt,
				EndAt:        reactionModels[i].Livestream_EndAt,
			},
			CreatedAt: reactionModels[i].Reaction_CreatedAt,
		}
	}

	return c.JSON(http.StatusOK, reactions)
}

func postReactionHandler(c echo.Context) error {
	ctx := c.Request().Context()
	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	var req *PostReactionRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to decode the request body as json")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	reactionModel := ReactionModel{
		UserID:       int64(userID),
		LivestreamID: int64(livestreamID),
		EmojiName:    req.EmojiName,
		CreatedAt:    time.Now().Unix(),
	}

	result, err := tx.NamedExecContext(ctx, "INSERT INTO reactions (user_id, livestream_id, emoji_name, created_at) VALUES (:user_id, :livestream_id, :emoji_name, :created_at)", reactionModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert reaction: "+err.Error())
	}

	reactionID, err := result.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get last inserted reaction id: "+err.Error())
	}
	reactionModel.ID = reactionID
	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}
	query := `
select
  reactions.id as "reaction_id"
  , reactions.emoji_name as "reaction_emoji_name"
  , reactions.created_at as "reaction_created_at"
  , users.id as "user_id"
  , users.name as "user_name"
  , users.display_name as "user_display_name"
  , users.description as "user_description"
  , themes.id as "theme_id"
  , themes.dark_mode as "theme_dark_mode"
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
from reactions
inner join users on users.id = reactions.user_id
inner join themes on themes.user_id = users.id
inner join livestreams on livestreams.id = reactions.livestream_id
inner join users as livestream_owners on livestream_owners.id = livestreams.user_id
inner join themes as livestream_owner_themes on livestream_owner_themes.user_id = livestream_owners.id
where reactions.id = ?
`
	reactionModel2 := ReactionModel2{}
	if err := dbConn.GetContext(ctx, &reactionModel2, query, reactionID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "failed to get reactions")
	}

	tags, err := getLivestreamTags2(ctx, reactionModel2.Livestream_ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
	}
	reaction := Reaction{
		ID:        reactionModel2.Reaction_ID,
		EmojiName: reactionModel2.Reaction_EmojiName,
		User: User{
			ID:          reactionModel2.User_ID,
			Name:        reactionModel2.User_Name,
			DisplayName: reactionModel2.User_DisplayName,
			Description: reactionModel2.User_Description,
			Theme: Theme{
				ID:       reactionModel2.Theme_ID,
				DarkMode: reactionModel2.Theme_DarkMode,
			},
			IconHash: getIconHashByUserId(reactionModel2.User_ID),
		},
		Livestream: Livestream{
			ID: reactionModel2.Livestream_ID,
			Owner: User{
				ID:          reactionModel2.LivestreamOwner_ID,
				Name:        reactionModel2.LivestreamOwner_Name,
				DisplayName: reactionModel2.LivestreamOwner_DisplayName,
				Description: reactionModel2.LivestreamOwner_Description,
				Theme: Theme{
					ID:       reactionModel2.LivestreamOwnerTheme_ID,
					DarkMode: reactionModel2.LivestreamOwnerTheme_DarkMode,
				},
				IconHash: getIconHashByUserId(reactionModel2.LivestreamOwner_ID),
			},
			Title:        reactionModel2.Livestream_Title,
			Description:  reactionModel2.Livestream_Description,
			PlaylistUrl:  reactionModel2.Livestream_PlaylistUrl,
			ThumbnailUrl: reactionModel2.Livestream_ThumbnailUrl,
			Tags:         tags,
			StartAt:      reactionModel2.Livestream_StartAt,
			EndAt:        reactionModel2.Livestream_EndAt,
		},
		CreatedAt: reactionModel2.Reaction_CreatedAt,
	}
	return c.JSON(http.StatusCreated, reaction)
}
