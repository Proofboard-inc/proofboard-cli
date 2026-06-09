package git

type Hook string

const (
	PostMerge   Hook = "post-merge"
	PostRewrite Hook = "post-rewrite"
)
