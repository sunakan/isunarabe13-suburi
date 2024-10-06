package main

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type TagModel struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type TagsResponse struct {
	Tags []*Tag `json:"tags"`
}

func getTagHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, &TagsResponse{
		Tags: tagsCache,
	})
}

// 配信者のテーマ取得API
// GET /api/user/:username/theme
func getStreamerThemeHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		c.Logger().Printf("verifyUserSession: %+v\n", err)
		return err
	}

	username := c.Param("username")

	query := `
select
  users.id as "user_id"
  , users.name as "user_name"
  , users.display_name as "user_display_name"
  , users.description as "user_description"
  , themes.id as "theme_id"
  , themes.dark_mode as "theme_dark_mode"
from users
inner join themes on themes.user_id = users.id
where users.name = ?
`
	userModel := UserModel2{}
	err := dbConn.GetContext(ctx, &userModel, query, username)
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "not found user that has the given username")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user: "+err.Error())
	}

	theme := Theme{
		ID:       userModel.ThemeID,
		DarkMode: userModel.ThemeDarkMode,
	}

	return c.JSON(http.StatusOK, theme)
}
