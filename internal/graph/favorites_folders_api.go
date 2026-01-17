package graph

import (
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
)

const favoriteFoldersDBKey = "favorite_folders"

func normalizeFavoriteFolderArgs(rootPath string, relativePath string) (root string, rel string) {
	root = filepath.Clean(rootPath)
	rel = filepath.Clean(relativePath)
	if rel == "." {
		rel = ""
	}
	return root, rel
}

func loadFavoriteFolders() []model.FavoriteFolder {
	var stored []model.FavoriteFolder
	_ = db.GetDefault().LoadJSON(favoriteFoldersDBKey, &stored)
	return stored
}

func storeFavoriteFolders(items []model.FavoriteFolder) {
	_ = db.GetDefault().StoreJSON(favoriteFoldersDBKey, items)
}

func addFavoriteFolder(rootPath string, relativePath string) (*model.FavoriteFolder, error) {
	root, rel := normalizeFavoriteFolderArgs(rootPath, relativePath)

	stored := loadFavoriteFolders()

	for _, f := range stored {
		if filepath.Clean(f.RootPath) == root && filepath.Clean(f.RelativePath) == rel {
			ff := f
			return &ff, nil
		}
	}

	newItem := model.FavoriteFolder{
		RootPath:     filepath.ToSlash(root),
		RelativePath: filepath.ToSlash(rel),
	}
	stored = append(stored, newItem)
	storeFavoriteFolders(stored)
	return &newItem, nil
}

func setFavoriteFolderAlias(rootPath string, relativePath string, alias string) (bool, error) {
	root, rel := normalizeFavoriteFolderArgs(rootPath, relativePath)
	alias = strings.TrimSpace(alias)

	stored := loadFavoriteFolders()

	changed := false
	for i := range stored {
		f := &stored[i]
		if filepath.Clean(f.RootPath) == root && filepath.Clean(f.RelativePath) == rel {
			if alias == "" {
				if f.Alias != nil {
					f.Alias = nil
					changed = true
				}
			} else {
				if f.Alias == nil || strings.TrimSpace(*f.Alias) != alias {
					v := alias
					f.Alias = &v
					changed = true
				}
			}
			break
		}
	}

	if changed {
		storeFavoriteFolders(stored)
	}

	return true, nil
}

func removeFavoriteFolder(rootPath string, relativePath string) (*model.FavoriteFolder, error) {
	root, rel := normalizeFavoriteFolderArgs(rootPath, relativePath)

	stored := loadFavoriteFolders()

	var removed *model.FavoriteFolder
	newList := make([]model.FavoriteFolder, 0, len(stored))
	for _, f := range stored {
		if filepath.Clean(f.RootPath) == root && filepath.Clean(f.RelativePath) == rel {
			ff := f
			removed = &ff
			continue
		}
		newList = append(newList, f)
	}
	storeFavoriteFolders(newList)

	if removed == nil {
		out := model.FavoriteFolder{RootPath: filepath.ToSlash(root), RelativePath: filepath.ToSlash(rel)}
		return &out, nil
	}
	return removed, nil
}

func favoriteFoldersList() []*model.FavoriteFolder {
	stored := loadFavoriteFolders()
	favorites := make([]*model.FavoriteFolder, 0, len(stored))
	for i := range stored {
		f := stored[i]
		f.RootPath = filepath.ToSlash(f.RootPath)
		f.RelativePath = filepath.ToSlash(f.RelativePath)
		if f.Alias != nil {
			v := strings.TrimSpace(*f.Alias)
			if v == "" {
				f.Alias = nil
			} else {
				f.Alias = &v
			}
		}
		favorites = append(favorites, &f)
	}
	return favorites
}
