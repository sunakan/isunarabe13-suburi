package main

import (
	"context"
	"github.com/jmoiron/sqlx"
	cmap "github.com/orcaman/concurrent-map/v2"
	"strconv"
	"sync"
	"sync/atomic"
)

var (
	tagsCache      []*Tag
	tagIdCache     = cmap.New[*Tag]()
	tagNameCache   = cmap.New[*Tag]()
	tagMutex       sync.Mutex
	tagId          atomic.Int64
	livestreamTags = cmap.New[[]*Tag]()
)

// tagsテーブルの代替
func initializeTagCache() {
	tagsCache = []*Tag{
		{ID: 1, Name: "ライブ配信"}, {ID: 2, Name: "ゲーム実況"}, {ID: 3, Name: "生放送"}, {ID: 4, Name: "アドバイス"}, {ID: 5, Name: "初心者歓迎"},
		{ID: 6, Name: "プロゲーマー"}, {ID: 7, Name: "新作ゲーム"}, {ID: 8, Name: "レトロゲーム"}, {ID: 9, Name: "RPG"}, {ID: 10, Name: "FPS"},
		{ID: 11, Name: "アクションゲーム"}, {ID: 12, Name: "対戦ゲーム"}, {ID: 13, Name: "マルチプレイ"}, {ID: 14, Name: "シングルプレイ"}, {ID: 15, Name: "ゲーム解説"},
		{ID: 16, Name: "ホラーゲーム"}, {ID: 17, Name: "イベント生放送"}, {ID: 18, Name: "新情報発表"}, {ID: 19, Name: "Q&Aセッション"}, {ID: 20, Name: "チャット交流"},
		{ID: 21, Name: "視聴者参加"}, {ID: 22, Name: "音楽ライブ"}, {ID: 23, Name: "カバーソング"}, {ID: 24, Name: "オリジナル楽曲"}, {ID: 25, Name: "アコースティック"},
		{ID: 26, Name: "歌配信"}, {ID: 27, Name: "楽器演奏"}, {ID: 28, Name: "ギター"}, {ID: 29, Name: "ピアノ"}, {ID: 30, Name: "バンドセッション"},
		{ID: 31, Name: "DJセット"}, {ID: 32, Name: "トーク配信"}, {ID: 33, Name: "朝活"}, {ID: 34, Name: "夜ふかし"}, {ID: 35, Name: "日常話"},
		{ID: 36, Name: "趣味の話"}, {ID: 37, Name: "語学学習"}, {ID: 38, Name: "お料理配信"}, {ID: 39, Name: "手料理"}, {ID: 40, Name: "レシピ紹介"},
		{ID: 41, Name: "アート配信"}, {ID: 42, Name: "絵描き"}, {ID: 43, Name: "DIY"}, {ID: 44, Name: "手芸"}, {ID: 45, Name: "アニメトーク"},
		{ID: 46, Name: "映画レビュー"}, {ID: 47, Name: "読書感想"}, {ID: 48, Name: "ファッション"}, {ID: 49, Name: "メイク"}, {ID: 50, Name: "ビューティー"},
		{ID: 51, Name: "健康"}, {ID: 52, Name: "ワークアウト"}, {ID: 53, Name: "ヨガ"}, {ID: 54, Name: "ダンス"}, {ID: 55, Name: "旅行記"},
		{ID: 56, Name: "アウトドア"}, {ID: 57, Name: "キャンプ"}, {ID: 58, Name: "ペットと一緒"}, {ID: 59, Name: "猫"}, {ID: 60, Name: "犬"},
		{ID: 61, Name: "釣り"}, {ID: 62, Name: "ガーデニング"}, {ID: 63, Name: "テクノロジー"}, {ID: 64, Name: "ガジェット紹介"}, {ID: 65, Name: "プログラミング"},
		{ID: 66, Name: "DIY電子工作"}, {ID: 67, Name: "ニュース解説"}, {ID: 68, Name: "歴史"}, {ID: 69, Name: "文化"}, {ID: 70, Name: "社会問題"},
		{ID: 71, Name: "心理学"}, {ID: 72, Name: "宇宙"}, {ID: 73, Name: "科学"}, {ID: 74, Name: "マジック"}, {ID: 75, Name: "コメディ"},
		{ID: 76, Name: "スポーツ"}, {ID: 77, Name: "サッカー"}, {ID: 78, Name: "野球"}, {ID: 79, Name: "バスケットボール"}, {ID: 80, Name: "ライフハック"},
		{ID: 81, Name: "教育"}, {ID: 82, Name: "子育て"}, {ID: 83, Name: "ビジネス"}, {ID: 84, Name: "起業"}, {ID: 85, Name: "投資"},
		{ID: 86, Name: "仮想通貨"}, {ID: 87, Name: "株式投資"}, {ID: 88, Name: "不動産"}, {ID: 89, Name: "キャリア"}, {ID: 90, Name: "スピリチュアル"},
		{ID: 91, Name: "占い"}, {ID: 92, Name: "手相"}, {ID: 93, Name: "オカルト"}, {ID: 94, Name: "UFO"}, {ID: 95, Name: "都市伝説"},
		{ID: 96, Name: "コンサート"}, {ID: 97, Name: "ファンミーティング"}, {ID: 98, Name: "コラボ配信"}, {ID: 99, Name: "記念配信"}, {ID: 100, Name: "生誕祭"},
		{ID: 101, Name: "周年記念"}, {ID: 102, Name: "サプライズ"}, {ID: 103, Name: "椅子"},
	}
	tagIdCache.Clear()
	tagNameCache.Clear()
	tagId.Store(103)
	for _, tag := range tagsCache {
		tagIdCache.Set(strconv.FormatInt(tag.ID, 10), tag)
		tagNameCache.Set(tag.Name, tag)
	}
}

// タグ名からTagを取得する
// キャッシュにない場合、atomic.AddInt64でIDをインクリメントして新規作成
// tagNameCacheに存在しない同じTagが同時にgetTagByNameされた場合、IDがむやみにインクリメントされる重複する可能性がある
// mutexを使って排他制御
func getPtrTagByName(name string) *Tag {
	if tag, ok := tagNameCache.Get(name); ok {
		return tag
	}
	// キャッシュにない場合、ロックを取得して、もう解除後はもう一度確認
	tagMutex.Lock()
	if ptrTag, ok := tagNameCache.Get(name); ok {
		return ptrTag
	}
	defer tagMutex.Unlock()
	ptrTag := &Tag{ID: tagId.Add(1), Name: name}
	tagNameCache.Set(name, ptrTag)
	return ptrTag
}

// タグIDからTagを取得
func getPtrTagByID(id int64) *Tag {
	if ptrTag, ok := tagIdCache.Get(strconv.FormatInt(id, 10)); ok {
		return ptrTag
	}
	return nil
}

// Livestreamに紐づくタグを取得
func getLivestreamTags(ctx context.Context, tx *sqlx.Tx, streamID int64) ([]Tag, error) {
	if ptrTags, ok := livestreamTags.Get(strconv.FormatInt(streamID, 10)); ok {
		tags := make([]Tag, len(ptrTags))
		for i, tag := range ptrTags {
			tags[i] = *tag
		}
		return tags, nil
	} else {
		tags := []Tag{}
		query := `select tags.* from tags inner join livestream_tags on tags.id = livestream_tags.tag_id where livestream_tags.livestream_id = ?;`
		if err := tx.SelectContext(ctx, &tags, query, streamID); err != nil {
			return nil, err
		}
		storedTags := make([]*Tag, len(tags))
		for i, tag := range tags {
			storedTags[i] = getPtrTagByID(tag.ID)
		}
		livestreamTags.Set(strconv.FormatInt(streamID, 10), storedTags)
		return tags, nil
	}
}
func getLivestreamTags2(ctx context.Context, streamID int64) ([]Tag, error) {
	if ptrTags, ok := livestreamTags.Get(strconv.FormatInt(streamID, 10)); ok {
		tags := make([]Tag, len(ptrTags))
		for i, tag := range ptrTags {
			tags[i] = *tag
		}
		return tags, nil
	} else {
		tags := []Tag{}
		query := `select tags.* from tags inner join livestream_tags on tags.id = livestream_tags.tag_id where livestream_tags.livestream_id = ?;`
		if err := dbConn.SelectContext(ctx, &tags, query, streamID); err != nil {
			return nil, err
		}
		storedTags := make([]*Tag, len(tags))
		for i, tag := range tags {
			storedTags[i] = getPtrTagByID(tag.ID)
		}
		livestreamTags.Set(strconv.FormatInt(streamID, 10), storedTags)
		return tags, nil
	}
}
