package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

type PostLivecommentRequest struct {
	Comment string `json:"comment"`
	Tip     int64  `json:"tip"`
}

type LivecommentModel struct {
	ID           int64  `db:"id"`
	UserID       int64  `db:"user_id"`
	LivestreamID int64  `db:"livestream_id"`
	Comment      string `db:"comment"`
	Tip          int64  `db:"tip"`
	CreatedAt    int64  `db:"created_at"`
}
type LivecommentModel2 struct {
	// livecomments
	Livecomment_ID        int64  `db:"livecomment_id"`
	Livecomment_Comment   string `db:"livecomment_comment"`
	Livecomment_Tip       int64  `db:"livecomment_tip"`
	Livecomment_CreatedAt int64  `db:"livecomment_created_at"`
	// users
	User_ID          int64  `db:"user_id"`
	User_Name        string `db:"user_name"`
	User_DisplayName string `db:"user_display_name"`
	User_Description string `db:"user_description"`
	// themes
	Theme_ID       int64 `db:"theme_id"`
	Theme_DarkMode bool  `db:"theme_dark_mode"`
	// icons
	Icon_Image []byte `db:"icon_image"`
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
	// livestream_owner_icons
	LivestreamOwnerIcon_Image []byte `db:"livestream_owner_icon_image"`
	// tags
	Livestream_Tags string `db:"livestream_tags"`
}
type Livecomment struct {
	ID         int64      `json:"id"`
	User       User       `json:"user"`
	Livestream Livestream `json:"livestream"`
	Comment    string     `json:"comment"`
	Tip        int64      `json:"tip"`
	CreatedAt  int64      `json:"created_at"`
}

type LivecommentReport struct {
	ID          int64       `json:"id"`
	Reporter    User        `json:"reporter"`
	Livecomment Livecomment `json:"livecomment"`
	CreatedAt   int64       `json:"created_at"`
}

type LivecommentReportModel struct {
	ID            int64 `db:"id"`
	UserID        int64 `db:"user_id"`
	LivestreamID  int64 `db:"livestream_id"`
	LivecommentID int64 `db:"livecomment_id"`
	CreatedAt     int64 `db:"created_at"`
}

type ModerateRequest struct {
	NGWord string `json:"ng_word"`
}

type NGWord struct {
	ID           int64  `json:"id" db:"id"`
	UserID       int64  `json:"user_id" db:"user_id"`
	LivestreamID int64  `json:"livestream_id" db:"livestream_id"`
	Word         string `json:"word" db:"word"`
	CreatedAt    int64  `json:"created_at" db:"created_at"`
}

func getLivecommentsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	// kaizen-01: 1発でもってくる
	// query := "SELECT * FROM livecomments WHERE livestream_id = ? ORDER BY created_at DESC"
	query := `select
livecomments.id as "livecomment_id"
  , livecomments.comment as "livecomment_comment"
  , livecomments.tip as "livecomment_tip"
  , livecomments.created_at as "livecomment_created_at"
  , users.id as "user_id"
  , users.name as "user_name"
  , users.display_name as "user_display_name"
  , users.description as "user_description"
  , themes.id as "theme_id"
  , themes.dark_mode as "theme_dark_mode"
  , icons.image as "icon_image"
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
  , livestream_owner_icons.image as "livestream_owner_icon_image"
  , IFNULL((select CONCAT('[', GROUP_CONCAT(CONCAT('{"id":', tags.id, ',"name":"', tags.name, '"}') SEPARATOR ','), ']') from livestream_tags inner join tags on livestream_tags.tag_id = tags.id where livestream_tags.livestream_id = livecomments.livestream_id), '[]') as "livestream_tags"
from livecomments
inner join users on users.id = livecomments.user_id
inner join themes on themes.user_id = users.id
left join icons on icons.user_id = users.id
inner join livestreams on livestreams.id = livecomments.livestream_id
inner join users as livestream_owners on livestream_owners.id = livestreams.user_id
inner join themes as livestream_owner_themes on livestream_owner_themes.user_id = livestream_owners.id
left join icons as livestream_owner_icons on livestream_owner_icons.user_id = livestream_owners.id
where livecomments.livestream_id = ?
order by livecomments.created_at desc
`
	if c.QueryParam("limit") != "" {
		limit, err := strconv.Atoi(c.QueryParam("limit"))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "limit query parameter must be integer")
		}
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	// kaizen-01: 1発で取得
	//livecommentModels := []LivecommentModel{}
	livecommentModels := []LivecommentModel2{}
	err = tx.SelectContext(ctx, &livecommentModels, query, livestreamID)
	if errors.Is(err, sql.ErrNoRows) {
		return c.JSON(http.StatusOK, []*Livecomment{})
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livecomments: "+err.Error())
	}

	// kaizen-01: tagsのUnmarshalは1回だけにして、使い回す
	var tags []Tag
	if len(livecommentModels) > 0 {
		err = json.Unmarshal([]byte(livecommentModels[0].Livestream_Tags), &tags)
		if err != nil {
			fmt.Println("JSONのデコードエラー:", err)
		}
	}

	livecomments := make([]Livecomment, len(livecommentModels))
	for i := range livecommentModels {
		// kaizen-01: 1発で取得したので、関数に投げない。ここでfillする
		// livecomment, err := fillLivecommentResponse(ctx, tx, livecommentModels[i])
		// if err != nil {
		// 	return echo.NewHTTPError(http.StatusInternalServerError, "failed to fil livecomments: "+err.Error())
		// }
		iconHash := fallbackImageHash
		livestreamOwnerIconHash := fallbackImageHash
		if len(livecommentModels[i].Icon_Image) > 0 {
			iconHash = fmt.Sprintf("%x", sha256.Sum256(livecommentModels[i].Icon_Image))
		}
		if len(livecommentModels[i].LivestreamOwnerIcon_Image) > 0 {
			livestreamOwnerIconHash = fmt.Sprintf("%x", sha256.Sum256(livecommentModels[i].LivestreamOwnerIcon_Image))
		}
		livecomments[i] = Livecomment{
			ID: livecommentModels[i].Livecomment_ID,
			User: User{
				ID:          livecommentModels[i].User_ID,
				Name:        livecommentModels[i].User_Name,
				DisplayName: livecommentModels[i].User_DisplayName,
				Description: livecommentModels[i].User_Description,
				Theme: Theme{
					ID:       livecommentModels[i].Theme_ID,
					DarkMode: livecommentModels[i].Theme_DarkMode,
				},
				IconHash: iconHash,
			},
			Livestream: Livestream{
				ID: livecommentModels[i].Livestream_ID,
				Owner: User{
					ID:          livecommentModels[i].LivestreamOwner_ID,
					Name:        livecommentModels[i].LivestreamOwner_Name,
					DisplayName: livecommentModels[i].LivestreamOwner_DisplayName,
					Description: livecommentModels[i].LivestreamOwner_Description,
					Theme: Theme{
						ID:       livecommentModels[i].LivestreamOwnerTheme_ID,
						DarkMode: livecommentModels[i].LivestreamOwnerTheme_DarkMode,
					},
					IconHash: livestreamOwnerIconHash,
				},
				Title:        livecommentModels[i].Livestream_Title,
				Description:  livecommentModels[i].Livestream_Description,
				PlaylistUrl:  livecommentModels[i].Livestream_PlaylistUrl,
				ThumbnailUrl: livecommentModels[i].Livestream_ThumbnailUrl,
				Tags:         tags,
				StartAt:      livecommentModels[i].Livestream_StartAt,
				EndAt:        livecommentModels[i].Livestream_EndAt,
			},
			Comment:   livecommentModels[i].Livecomment_Comment,
			Tip:       livecommentModels[i].Livecomment_Tip,
			CreatedAt: livecommentModels[i].Livecomment_CreatedAt,
		}
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, livecomments)
}

func getNgwords(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
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

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	var ngWords []*NGWord
	if err := tx.SelectContext(ctx, &ngWords, "SELECT * FROM ng_words WHERE user_id = ? AND livestream_id = ? ORDER BY created_at DESC", userID, livestreamID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusOK, []*NGWord{})
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get NG words: "+err.Error())
		}
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, ngWords)
}

func postLivecommentHandler(c echo.Context) error {
	ctx := c.Request().Context()
	defer c.Request().Body.Close()

	if err := verifyUserSession(c); err != nil {
		return err
	}

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	var req *PostLivecommentRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to decode the request body as json")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	var livestreamModel LivestreamModel
	if err := tx.GetContext(ctx, &livestreamModel, "SELECT * FROM livestreams WHERE id = ?", livestreamID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "livestream not found")
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream: "+err.Error())
		}
	}

	// スパム判定
	var ngwords []*NGWord
	if err := tx.SelectContext(ctx, &ngwords, "SELECT id, user_id, livestream_id, word FROM ng_words WHERE user_id = ? AND livestream_id = ?", livestreamModel.UserID, livestreamModel.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get NG words: "+err.Error())
	}

	var hitSpam int
	for _, ngword := range ngwords {
		query := `
		SELECT COUNT(*)
		FROM
		(SELECT ? AS text) AS texts
		INNER JOIN
		(SELECT CONCAT('%', ?, '%')	AS pattern) AS patterns
		ON texts.text LIKE patterns.pattern;
		`
		if err := tx.GetContext(ctx, &hitSpam, query, req.Comment, ngword.Word); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get hitspam: "+err.Error())
		}
		c.Logger().Infof("[hitSpam=%d] comment = %s", hitSpam, req.Comment)
		if hitSpam >= 1 {
			return echo.NewHTTPError(http.StatusBadRequest, "このコメントがスパム判定されました")
		}
	}

	now := time.Now().Unix()
	livecommentModel := LivecommentModel{
		UserID:       userID,
		LivestreamID: int64(livestreamID),
		Comment:      req.Comment,
		Tip:          req.Tip,
		CreatedAt:    now,
	}

	rs, err := tx.NamedExecContext(ctx, "INSERT INTO livecomments (user_id, livestream_id, comment, tip, created_at) VALUES (:user_id, :livestream_id, :comment, :tip, :created_at)", livecommentModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert livecomment: "+err.Error())
	}

	livecommentID, err := rs.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get last inserted livecomment id: "+err.Error())
	}
	livecommentModel.ID = livecommentID

	livecomment, err := fillLivecommentResponse(ctx, tx, livecommentModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill livecomment: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusCreated, livecomment)
}

func reportLivecommentHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		return err
	}

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	livecommentID, err := strconv.Atoi(c.Param("livecomment_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livecomment_id in path must be integer")
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	var livestreamModel LivestreamModel
	if err := tx.GetContext(ctx, &livestreamModel, "SELECT * FROM livestreams WHERE id = ?", livestreamID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "livestream not found")
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream: "+err.Error())
		}
	}

	var livecommentModel LivecommentModel
	if err := tx.GetContext(ctx, &livecommentModel, "SELECT * FROM livecomments WHERE id = ?", livecommentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "livecomment not found")
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livecomment: "+err.Error())
		}
	}

	now := time.Now().Unix()
	reportModel := LivecommentReportModel{
		UserID:        int64(userID),
		LivestreamID:  int64(livestreamID),
		LivecommentID: int64(livecommentID),
		CreatedAt:     now,
	}
	rs, err := tx.NamedExecContext(ctx, "INSERT INTO livecomment_reports(user_id, livestream_id, livecomment_id, created_at) VALUES (:user_id, :livestream_id, :livecomment_id, :created_at)", &reportModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert livecomment report: "+err.Error())
	}
	reportID, err := rs.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get last inserted livecomment report id: "+err.Error())
	}
	reportModel.ID = reportID

	report, err := fillLivecommentReportResponse(ctx, tx, reportModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill livecomment report: "+err.Error())
	}
	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusCreated, report)
}

// NGワードを登録
func moderateHandler(c echo.Context) error {
	ctx := c.Request().Context()
	defer c.Request().Body.Close()

	if err := verifyUserSession(c); err != nil {
		return err
	}

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	var req *ModerateRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to decode the request body as json")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	// 配信者自身の配信に対するmoderateなのかを検証
	var ownedLivestreams []LivestreamModel
	if err := tx.SelectContext(ctx, &ownedLivestreams, "SELECT * FROM livestreams WHERE id = ? AND user_id = ?", livestreamID, userID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestreams: "+err.Error())
	}
	if len(ownedLivestreams) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "A streamer can't moderate livestreams that other streamers own")
	}

	rs, err := tx.NamedExecContext(ctx, "INSERT INTO ng_words(user_id, livestream_id, word, created_at) VALUES (:user_id, :livestream_id, :word, :created_at)", &NGWord{
		UserID:       int64(userID),
		LivestreamID: int64(livestreamID),
		Word:         req.NGWord,
		CreatedAt:    time.Now().Unix(),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert new NG word: "+err.Error())
	}

	wordID, err := rs.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get last inserted NG word id: "+err.Error())
	}

	// kaizen-03: 1発で削除
	// kaizen-03: 過去分は既に削除済みなので、NGワードを改めて全て取る必要はない(事前にコメントしようとすると弾かれる)
	//var ngwords []*NGWord
	//if err := tx.SelectContext(ctx, &ngwords, "SELECT * FROM ng_words WHERE livestream_id = ?", livestreamID); err != nil {
	//	return echo.NewHTTPError(http.StatusInternalServerError, "failed to get NG words: "+err.Error())
	//}
	//// NGワードにヒットする過去の投稿も全削除する
	//for _, ngword := range ngwords {
	//	// ライブコメント一覧取得
	//	var livecomments []*LivecommentModel
	//	if err := tx.SelectContext(ctx, &livecomments, "SELECT * FROM livecomments"); err != nil {
	//		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livecomments: "+err.Error())
	//	}
	//	for _, livecomment := range livecomments {
	//		query := `
	//		DELETE FROM livecomments
	//		WHERE
	//		id = ? AND
	//		livestream_id = ? AND
	//		(SELECT COUNT(*)
	//		FROM
	//		(SELECT ? AS text) AS texts
	//		INNER JOIN
	//		(SELECT CONCAT('%', ?, '%')	AS pattern) AS patterns
	//		ON texts.text LIKE patterns.pattern) >= 1;
	//		`
	//		if _, err := tx.ExecContext(ctx, query, livecomment.ID, livestreamID, livecomment.Comment, ngword.Word); err != nil {
	//			return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete old livecomments that hit spams: "+err.Error())
	//		}
	//	}
	//}
	if _, err := tx.ExecContext(ctx, "delete from livecomments where livestream_id = ? and comment like ?;", livestreamID, "%"+req.NGWord+"%"); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete old livecomments that hit spams: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"word_id": wordID,
	})
}

func fillLivecommentResponse(ctx context.Context, tx *sqlx.Tx, livecommentModel LivecommentModel) (Livecomment, error) {
	commentOwnerModel := UserModel{}
	if err := tx.GetContext(ctx, &commentOwnerModel, "SELECT * FROM users WHERE id = ?", livecommentModel.UserID); err != nil {
		return Livecomment{}, err
	}
	commentOwner, err := fillUserResponse(ctx, tx, commentOwnerModel)
	if err != nil {
		return Livecomment{}, err
	}

	livestreamModel := LivestreamModel{}
	if err := tx.GetContext(ctx, &livestreamModel, "SELECT * FROM livestreams WHERE id = ?", livecommentModel.LivestreamID); err != nil {
		return Livecomment{}, err
	}
	livestream, err := fillLivestreamResponse(ctx, tx, livestreamModel)
	if err != nil {
		return Livecomment{}, err
	}

	livecomment := Livecomment{
		ID:         livecommentModel.ID,
		User:       commentOwner,
		Livestream: livestream,
		Comment:    livecommentModel.Comment,
		Tip:        livecommentModel.Tip,
		CreatedAt:  livecommentModel.CreatedAt,
	}

	return livecomment, nil
}

func fillLivecommentReportResponse(ctx context.Context, tx *sqlx.Tx, reportModel LivecommentReportModel) (LivecommentReport, error) {
	reporterModel := UserModel{}
	if err := tx.GetContext(ctx, &reporterModel, "SELECT * FROM users WHERE id = ?", reportModel.UserID); err != nil {
		return LivecommentReport{}, err
	}
	reporter, err := fillUserResponse(ctx, tx, reporterModel)
	if err != nil {
		return LivecommentReport{}, err
	}

	livecommentModel := LivecommentModel{}
	if err := tx.GetContext(ctx, &livecommentModel, "SELECT * FROM livecomments WHERE id = ?", reportModel.LivecommentID); err != nil {
		return LivecommentReport{}, err
	}
	livecomment, err := fillLivecommentResponse(ctx, tx, livecommentModel)
	if err != nil {
		return LivecommentReport{}, err
	}

	report := LivecommentReport{
		ID:          reportModel.ID,
		Reporter:    reporter,
		Livecomment: livecomment,
		CreatedAt:   reportModel.CreatedAt,
	}
	return report, nil
}
