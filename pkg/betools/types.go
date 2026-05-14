package betools

type ContentTarget struct {
	Kind  string
	Value string
}

type TargetSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type BatchEditSpec struct {
	Action  string `json:"action"`
	Start   any    `json:"start,omitempty"`
	End     any    `json:"end,omitempty"`
	Line    any    `json:"line,omitempty"`
	Content string `json:"content,omitempty"`
}

type BatchFileSpec struct {
	File  string          `json:"file"`
	Edits []BatchEditSpec `json:"edits"`
}

type ShowResult struct {
	Status  string `json:"status"`
	File    string `json:"file"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Total   int    `json:"total"`
	Content string `json:"content"`
	Brief    bool     `json:"brief,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type ReplaceResult struct {
	Status   string   `json:"status"`
	File     string   `json:"file"`
	Removed  int      `json:"removed"`
	Added    int      `json:"added"`
	Total    int      `json:"total"`
	Diff     string   `json:"diff"`
	Balance  string   `json:"balance"`
	Affected string   `json:"affected"`
	Preview  bool     `json:"preview,omitempty"`
	Warning  string   `json:"warning,omitempty"`
	Brief    bool     `json:"brief,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	EventID  string   `json:"event_id,omitempty"`
	QueueFull bool    `json:"queue_full,omitempty"`
}

type InsertResult struct {
	Status   string   `json:"status"`
	File     string   `json:"file"`
	After    int      `json:"after"`
	Added    int      `json:"added"`
	Total    int      `json:"total"`
	Diff     string   `json:"diff"`
	Balance  string   `json:"balance"`
	Affected string   `json:"affected"`
	Preview  bool     `json:"preview,omitempty"`
	Brief    bool     `json:"brief,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	EventID  string   `json:"event_id,omitempty"`
	QueueFull bool    `json:"queue_full,omitempty"`
}

type DeleteResult struct {
	Status   string   `json:"status"`
	File     string   `json:"file"`
	Total    int      `json:"total"`
	Diff     string   `json:"diff"`
	Balance  string   `json:"balance"`
	Affected string   `json:"affected"`
	Preview  bool     `json:"preview,omitempty"`
	Brief    bool     `json:"brief,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	EventID  string   `json:"event_id,omitempty"`
	QueueFull bool    `json:"queue_full,omitempty"`
}

type BatchFileResult struct {
	File     string   `json:"file"`
	Edits    int      `json:"edits"`
	Total    int      `json:"total"`
	Warnings []string `json:"warnings,omitempty"`
}

type BatchResult struct {
	Status  string            `json:"status"`
	Files   int               `json:"files"`
	Results []BatchFileResult `json:"results"`
	Preview bool              `json:"preview,omitempty"`
	Brief   bool              `json:"brief,omitempty"`
	EventID string            `json:"event_id,omitempty"`
	QueueFull bool            `json:"queue_full,omitempty"`
}

type WriteSpecItem struct {
	File    string
	Content string
}

type WriteFileResult struct {
	File     string   `json:"file"`
	Lines    int      `json:"lines"`
	Bytes    int      `json:"bytes"`
	Warnings []string `json:"warnings,omitempty"`
}

type WriteResult struct {
	Status   string            `json:"status"`
	Files    int               `json:"files"`
	Results  []WriteFileResult `json:"results"`
	Degraded bool              `json:"degraded,omitempty"`
	Warning  string            `json:"warning,omitempty"`
	Preview  bool              `json:"preview,omitempty"`
	Brief    bool              `json:"brief,omitempty"`
	EventID  string            `json:"event_id,omitempty"`
	QueueFull bool             `json:"queue_full,omitempty"`
}

type FunctionRangeResult struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type TagRangeResult struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Kind  string `json:"kind"`
	Tag   string `json:"tag,omitempty"`
}

type MatchedPair struct {
	Symbol    string `json:"symbol"`
	OpenLine  int    `json:"open_line"`
	CloseLine int    `json:"close_line"`
	Depth     int    `json:"depth"`
}

type UnbalancedItem struct {
	Symbol   string `json:"symbol"`
	Line     int    `json:"line"`
	Expected string `json:"expected"`
}

type QuoteWarning struct {
	Symbol string `json:"symbol"`
	Count  int    `json:"count"`
	Lines  []int  `json:"lines"`
}
