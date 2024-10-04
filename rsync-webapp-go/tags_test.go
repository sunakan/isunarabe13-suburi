package main

import "testing"

func TestGetPtrTagByID(t *testing.T) {
	testCases := []struct {
		name string
		id   int64
		want Tag
	}{
		{
			name: "IDが1の場合",
			id:   1,
			want: Tag{
				ID:   1,
				Name: "ライブ配信",
			},
		},
		{
			name: "IDが2の場合",
			id:   2,
			want: Tag{
				ID:   2,
				Name: "ゲーム実況",
			},
		},
		{
			name: "IDが103の場合",
			id:   103,
			want: Tag{
				ID:   103,
				Name: "椅子",
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got := getPtrTagByID(tt.id)
			if *got != tt.want {
				t.Errorf("got: %v, want: %v", *got, tt.want)
			}
		})
	}
}
