package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type LivestreamStatistics struct {
	Rank           int64 `json:"rank" db:"rank"`
	ViewersCount   int64 `json:"viewers_count" db:"viewers_count"`
	TotalReactions int64 `json:"total_reactions" db:"total_reactions"`
	TotalReports   int64 `json:"total_reports" db:"total_reports"`
	MaxTip         int64 `json:"max_tip" db:"max_tip"`
}

type UserStatistics struct {
	Rank              int64  `json:"rank" db:"rank"`
	ViewersCount      int64  `json:"viewers_count" db:"viewers_count"`
	TotalReactions    int64  `json:"total_reactions" db:"total_reactions"`
	TotalLivecomments int64  `json:"total_livecomments" db:"total_livecomments"`
	TotalTip          int64  `json:"total_tip" db:"total_tip"`
	FavoriteEmoji     string `json:"favorite_emoji" db:"favorite_emoji"`
}

func getUserStatisticsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	username := c.Param("username")
	// ユーザごとに、紐づく配信について、累計リアクション数、累計ライブコメント数、累計売上金額を算出
	// また、現在の合計視聴者数もだす

	// kaizen-03: 1発で取得
	// スコア: reactions(INSERTされるだけ) + tips(時々NgWordで設定によって、削除されて減る)
	query := `
with scores as (
select
  users.id as user_id
  , users.name as user_name
  , IFNULL((select count(1) from livestreams inner join reactions on reactions.livestream_id = livestreams.id where livestreams.user_id = users.id), 0) as total_reactions
  , IFNULL((select sum(livecomments.tip) from livestreams inner join livecomments on livecomments.livestream_id = livestreams.id where livestreams.user_id = users.id), 0) as total_tip
from users
), user_ranking as (
select
  scores.user_id as user_id
  , scores.user_name
  , scores.total_reactions
  , scores.total_tip
  , ROW_NUMBER() over (order by (scores.total_reactions+scores.total_tip) desc, scores.user_name desc) as "rank"
from scores
)
select
  user_ranking.rank
  , user_ranking.total_reactions
  , (select count(1) from livestreams inner join livecomments on livecomments.livestream_id = livestreams.id where livestreams.user_id = user_ranking.user_id) as total_livecomments
  , user_ranking.total_tip
  , (select count(1) from livestreams inner join livestream_viewers_history on livestream_viewers_history.livestream_id = livestreams.id where livestreams.user_id = user_ranking.user_id) as viewers_count
  , IFNULL((select reactions.emoji_name from livestreams inner join reactions on reactions.livestream_id = livestreams.id where livestreams.user_id = user_ranking.user_id group by reactions.emoji_name order by count(1) desc, reactions.emoji_name desc limit 1), '') as favorite_emoji
from user_ranking
where user_ranking.user_name = ?
;
`
	stats := UserStatistics{}
	if err := dbConn.GetContext(ctx, &stats, query, username); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get stats: "+err.Error())
	}
	return c.JSON(http.StatusOK, stats)
}

// livestream_idに紐づく配信の統計情報を取得
// id: livestream_id
func getLivestreamStatisticsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		return err
	}

	id, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}
	livestreamID := int64(id)

	// kaizen-08: Livestreamの統計情報を1発で取得
	// ランク = reactions数 + tips数の合計で降順(同点だった場合、LivestreamIDの降順)
	// 視聴者数 = livestream_viewers_history数
	// 合計リアクション数 = reactions数
	// 最大チップ額 = livecommentsのtipの最大値(ない場合、0)
	// レポート数 = livecomment_reports数
	query := `
with scores as (
select
  livestreams.id as livestream_id
  , IFNULL((select count(1) from reactions where reactions.livestream_id = livestreams.id), 0) as total_reactions
  , IFNULL((select sum(livecomments.tip) from livecomments where livecomments.livestream_id = livestreams.id), 0) as total_tip
from livestreams
), livestream_ranking as (
select
  scores.livestream_id as livestream_id
  , scores.total_reactions
  , scores.total_tip
  , ROW_NUMBER() over (order by (scores.total_reactions+scores.total_tip) desc, scores.livestream_id desc) as "rank"
from scores
)
select
  livestream_ranking.rank
  , (select count(1) from livestream_viewers_history where livestream_viewers_history.livestream_id = livestream_ranking.livestream_id) as viewers_count
  , IFNULL((select max(livecomments.tip) from livecomments where livecomments.livestream_id = livestream_ranking.livestream_id), 0) as max_tip
  , livestream_ranking.total_reactions as total_reactions
  , IFNULL((select count(1) from livecomment_reports where livecomment_reports.livestream_id = livestream_ranking.livestream_id), 0) as total_reports
from livestream_ranking
where livestream_ranking.livestream_id = ?
`
	stats := LivestreamStatistics{}
	if err := dbConn.GetContext(ctx, &stats, query, livestreamID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get stats: "+err.Error())
	}
	return c.JSON(http.StatusOK, stats)
}
