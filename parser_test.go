package schemalex_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy"
	"github.com/shogo82148/schemalex-deploy/model"
)

func TestParseError1(t *testing.T) {
	const src = "CREATE TABLE foo (id int PRIMARY KEY);\nCREATE TABLE bar"
	p := schemalex.New()
	_, err := p.ParseString(src)
	if err == nil {
		t.Fatal("parse should fail")
	}

	expected := "parse error: expected LPAREN at line 2 column 16 (at EOF)\n" +
		"    \"CREATE TABLE bar\" <---- AROUND HERE"
	if diff := cmp.Diff(err.Error(), expected); diff != "" {
		t.Errorf("unexpected error message: (-want/+got):\n%s", diff)
	}
}

func TestParseError2(t *testing.T) {
	const src = "CREATE TABLE foo (id int PRIMARY KEY);\nCREATE TABLE bar (id int PRIMARY KEY baz TEXT)"
	p := schemalex.New()
	_, err := p.ParseString(src)
	if err == nil {
		t.Fatal("parse should fail")
	}

	expected := "parse error: unexpected column option IDENT at line 2 column 37\n" +
		"    \"CREATE TABLE bar (id int PRIMARY KEY \" <---- AROUND HERE"
	if diff := cmp.Diff(err.Error(), expected); diff != "" {
		t.Errorf("unexpected error message: (-want/+got):\n%s", diff)
	}
}

func TestParse1(t *testing.T) {
	tests := []struct {
		src  string
		want model.Stmts
	}{
		{
			src: "CREATE TABLE `fuga` (\n" +
				"`id` INTEGER NOT NULL AUTO_INCREMENT,\n" +
				"PRIMARY KEY (`id`),\n" +
				"`fid` INTEGER NOT NULL,\n" +
				"FOREIGN KEY fk (fid) REFERENCES f (id) ON DELETE CASCADE ON UPDATE CASCADE );",
			want: model.Stmts{
				&model.Table{
					Name: "fuga",
					Columns: []*model.TableColumn{
						{
							Name:          "id",
							Type:          model.ColumnTypeInt,
							Length:        model.NewLength("11"),
							NullState:     model.NullStateNotNull,
							AutoIncrement: true,
						},
						{
							Name:      "fid",
							Type:      model.ColumnTypeInt,
							Length:    model.NewLength("11"),
							NullState: model.NullStateNotNull,
						},
					},
					Indexes: []*model.Index{
						{
							Table: "table#fuga",
							Kind:  model.IndexKindPrimaryKey,
							Columns: []*model.IndexColumn{
								{Name: "id"},
							},
						},
						{
							Table: "table#fuga",
							Kind:  model.IndexKindForeignKey,
							Name: model.MaybeIdent{
								Ident: "fk",
								Valid: true,
							},
							Columns: []*model.IndexColumn{
								{Name: "fid"},
							},
							Reference: &model.Reference{
								TableName: "f",
								Columns: []*model.IndexColumn{
									{Name: "id"},
								},
								OnDelete: model.ReferenceOptionCascade,
								OnUpdate: model.ReferenceOptionCascade,
							},
						},
					},
					Options: []*model.TableOption{},
				},
			},
		},
		{
			src: "CREATE TABLE `fuga` (\n" +
				"`id` INTEGER NOT NULL AUTO_INCREMENT,\n" +
				"PRIMARY KEY (`id`),\n" +
				"`fid` INTEGER NOT NULL,\n" +
				"FOREIGN KEY fk (fid) REFERENCES f (id) ON UPDATE CASCADE ON DELETE CASCADE);",
			want: model.Stmts{
				&model.Table{
					Name: "fuga",
					Columns: []*model.TableColumn{
						{
							Name:          "id",
							Type:          model.ColumnTypeInt,
							Length:        model.NewLength("11"),
							NullState:     model.NullStateNotNull,
							AutoIncrement: true,
						},
						{
							Name:      "fid",
							Type:      model.ColumnTypeInt,
							Length:    model.NewLength("11"),
							NullState: model.NullStateNotNull,
						},
					},
					Indexes: []*model.Index{
						{
							Table: "table#fuga",
							Kind:  model.IndexKindPrimaryKey,
							Columns: []*model.IndexColumn{
								{Name: "id"},
							},
						},
						{
							Table: "table#fuga",
							Kind:  model.IndexKindForeignKey,
							Name: model.MaybeIdent{
								Ident: "fk",
								Valid: true,
							},
							Columns: []*model.IndexColumn{
								{Name: "fid"},
							},
							Reference: &model.Reference{
								TableName: "f",
								Columns: []*model.IndexColumn{
									{Name: "id"},
								},
								OnDelete: model.ReferenceOptionCascade,
								OnUpdate: model.ReferenceOptionCascade,
							},
						},
					},
					Options: []*model.TableOption{},
				},
			},
		},
		{
			src: "CREATE TABLE `fuga` (\n" +
				"`id` INTEGER NOT NULL AUTO_INCREMENT,\n" +
				"`point` POINT SRID 4326 NOT NULL,\n" +
				"SPATIAL KEY `idx_point` (`point`),\n" +
				"PRIMARY KEY (`id`)\n" +
				");",
			want: model.Stmts{
				&model.Table{
					Name: "fuga",
					Columns: []*model.TableColumn{
						{
							Name:          "id",
							Type:          model.ColumnTypeInt,
							Length:        model.NewLength("11"),
							NullState:     model.NullStateNotNull,
							AutoIncrement: true,
						},
						{
							Name:      "point",
							Type:      model.ColumnTypePoint,
							NullState: model.NullStateNotNull,
							SRID: model.MaybeInteger{
								Valid: true,
								Value: 4326,
							},
						},
					},
					Indexes: []*model.Index{
						{
							Table: "table#fuga",
							Kind:  model.IndexKindSpatial,
							Name: model.MaybeIdent{
								Valid: true,
								Ident: "idx_point",
							},
							Columns: []*model.IndexColumn{
								{Name: "point"},
							},
						},
						{
							Table: "table#fuga",
							Kind:  model.IndexKindPrimaryKey,
							Columns: []*model.IndexColumn{
								{Name: "id"},
							},
						},
					},
					Options: []*model.TableOption{},
				},
			},
		},
		{
			src: "CREATE TABLE `fuga` (\n" +
				"`id` INTEGER NOT NULL AUTO_INCREMENT,\n" +
				"`title` TEXT NOT NULL,\n" +
				"FULLTEXT INDEX `title_idx` (`title`) WITH PARSER `ngram`,\n" +
				"PRIMARY KEY (`id`)\n" +
				");",
			want: model.Stmts{
				&model.Table{
					Name: "fuga",
					Columns: []*model.TableColumn{
						{
							Name:          "id",
							Type:          model.ColumnTypeInt,
							Length:        model.NewLength("11"),
							NullState:     model.NullStateNotNull,
							AutoIncrement: true,
						},
						{
							Name:      "title",
							Type:      model.ColumnTypeText,
							NullState: model.NullStateNotNull,
						},
					},
					Indexes: []*model.Index{
						{
							Table: "table#fuga",
							Kind:  model.IndexKindFullText,
							Name: model.MaybeIdent{
								Valid: true,
								Ident: "title_idx",
							},
							Columns: []*model.IndexColumn{
								{Name: "title"},
							},
							Options: []*model.IndexOption{
								{
									Key:        "WITH PARSER",
									Value:      "ngram",
									NeedQuotes: true,
								},
							},
						},
						{
							Table: "table#fuga",
							Kind:  model.IndexKindPrimaryKey,
							Columns: []*model.IndexColumn{
								{Name: "id"},
							},
						},
					},
					Options: []*model.TableOption{},
				},
			},
		},
	}
	for _, tt := range tests {
		p := schemalex.New()
		got, err := p.ParseString(tt.src)
		if err != nil {
			t.Errorf("while parsing %q, got an error: %v", tt.src, err)
			continue
		}
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("while parsing %q, got unexpected result:\n(-want/+got):\n%s", tt.src, diff)
		}
	}

}
