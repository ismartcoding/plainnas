package db

import (
  "encoding/json"
  "path/filepath"
  "sort"
  "strings"
)

const mediaSourceDirsKey = "settings:media_source_dirs"

func GetMediaSourceDirs() []string {
  raw, err := GetDefault().Get([]byte(mediaSourceDirsKey))
  if err != nil || len(raw) == 0 {
    return nil
  }
  var dirs []string
  if err := json.Unmarshal(raw, &dirs); err != nil {
    return nil
  }
  return normalizeDirs(dirs)
}

func SetMediaSourceDirs(dirs []string) {
  clean := normalizeDirs(dirs)
  if len(clean) == 0 {
    DeleteValue(mediaSourceDirsKey)
    return
  }
  b, err := json.Marshal(clean)
  if err != nil {
    return
  }
  SetValue(mediaSourceDirsKey, string(b))
}

func normalizeDirs(dirs []string) []string {
  if len(dirs) == 0 {
    return nil
  }
  seen := map[string]struct{}{}
  out := make([]string, 0, len(dirs))
  for _, d := range dirs {
    d = filepath.ToSlash(filepath.Clean(strings.TrimSpace(d)))
    if d == "." || d == "" {
      continue
    }
    if _, ok := seen[d]; ok {
      continue
    }
    seen[d] = struct{}{}
    out = append(out, d)
  }
  sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
  return out
}
