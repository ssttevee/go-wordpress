package wordpress

type AfterGetAttachmentsFilterFunc func(*WordPress, []*Attachment) ([]*Attachment, error)
type AfterGetPostsFilterFunc func(*WordPress, []*Post) ([]*Post, error)

var (
	filters struct {
		AfterGetAttachments []AfterGetAttachmentsFilterFunc
		AfterGetPosts       []AfterGetPostsFilterFunc
	}
)

func AddAfterGetAttachmentsFilter(f AfterGetAttachmentsFilterFunc) {
	filters.AfterGetAttachments = append(filters.AfterGetAttachments, f)
}

func AddAfterGetPostsFilter(f AfterGetPostsFilterFunc) {
	filters.AfterGetPosts = append(filters.AfterGetPosts, f)
}