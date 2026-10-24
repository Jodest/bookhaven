package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	ratingsDomain "github.com/bookhaven/backend/internal/modules/ratings/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type bookUserKey struct {
	bookID uuid.UUID
	userID uuid.UUID
}

type reviewUserKey struct {
	reviewID uuid.UUID
	userID   uuid.UUID
}

// fakeNotification mirrors the one notification row a (actor, review) pair
// can ever have, per the notifications table's UNIQUE (actor_id, review_id,
// type) upsert constraint (see reviews/repository.GormRepository.ToggleLike
// and its upsertNotificationInTx).
type fakeNotification struct {
	IsRead    bool
	CreatedAt time.Time
}

type fakeRepository struct {
	reviews       map[uuid.UUID]*reviewsDomain.Review
	ratings       map[bookUserKey]*ratingsDomain.Rating
	likes         map[reviewUserKey]bool
	books         map[uuid.UUID]*catalogDomain.Book
	notifications map[reviewUserKey]*fakeNotification
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		reviews:       make(map[uuid.UUID]*reviewsDomain.Review),
		ratings:       make(map[bookUserKey]*ratingsDomain.Rating),
		likes:         make(map[reviewUserKey]bool),
		books:         make(map[uuid.UUID]*catalogDomain.Book),
		notifications: make(map[reviewUserKey]*fakeNotification),
	}
}

func (f *fakeRepository) recalc(bookID uuid.UUID) {
	book, ok := f.books[bookID]
	if !ok {
		return
	}

	var sum int64
	ratingsCount := 0
	for _, r := range f.ratings {
		if r.BookID == bookID {
			sum += int64(r.Value)
			ratingsCount++
		}
	}
	if ratingsCount > 0 {
		book.Rating = float64(sum) / float64(ratingsCount)
	} else {
		book.Rating = 0
	}
	book.RatingsCount = ratingsCount

	reviewsCount := 0
	for _, r := range f.reviews {
		if r.BookID == bookID && !r.IsHidden {
			reviewsCount++
		}
	}
	book.ReviewsCount = reviewsCount
}

func (f *fakeRepository) upsertRating(bookID, userID uuid.UUID, value int) {
	key := bookUserKey{bookID: bookID, userID: userID}
	if r, ok := f.ratings[key]; ok {
		r.Value = value
	} else {
		f.ratings[key] = &ratingsDomain.Rating{
			ID:     uuid.New(),
			BookID: bookID,
			UserID: userID,
			Value:  value,
		}
	}
}

func (f *fakeRepository) ListByBook(ctx context.Context, bookID uuid.UUID, userID *uuid.UUID, limit, offset int) ([]reviewsDomain.Review, int64, error) {
	var visible []reviewsDomain.Review
	for _, r := range f.reviews {
		if r.BookID == bookID && !r.IsHidden {
			copy := *r
			if userID != nil {
				copy.LikedByMe = f.likes[reviewUserKey{reviewID: copy.ID, userID: *userID}]
			}
			visible = append(visible, copy)
		}
	}

	orderDesc(visible)

	total := int64(len(visible))
	if offset >= len(visible) {
		return []reviewsDomain.Review{}, total, nil
	}
	end := offset + limit
	if end > len(visible) {
		end = len(visible)
	}
	return visible[offset:end], total, nil
}

func orderDesc(reviews []reviewsDomain.Review) {
	for i := 0; i < len(reviews)-1; i++ {
		for j := i + 1; j < len(reviews); j++ {
			if reviews[i].CreatedAt.Before(reviews[j].CreatedAt) {
				reviews[i], reviews[j] = reviews[j], reviews[i]
			}
		}
	}
}

func (f *fakeRepository) GetByID(ctx context.Context, reviewID uuid.UUID) (*reviewsDomain.Review, error) {
	r, ok := f.reviews[reviewID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *r
	return &copy, nil
}

func (f *fakeRepository) GetByUserAndBook(ctx context.Context, userID, bookID uuid.UUID) (*reviewsDomain.Review, error) {
	for _, r := range f.reviews {
		if r.BookID == bookID && r.UserID == userID {
			copy := *r
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) Create(ctx context.Context, review *reviewsDomain.Review) error {
	copy := *review
	f.reviews[review.ID] = &copy
	f.upsertRating(review.BookID, review.UserID, review.Rating)
	f.recalc(review.BookID)
	return nil
}

func (f *fakeRepository) Update(ctx context.Context, review *reviewsDomain.Review) error {
	if _, ok := f.reviews[review.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	copy := *review
	f.reviews[review.ID] = &copy
	f.upsertRating(review.BookID, review.UserID, review.Rating)
	f.recalc(review.BookID)
	return nil
}

func (f *fakeRepository) Delete(ctx context.Context, reviewID uuid.UUID) error {
	r, ok := f.reviews[reviewID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	delete(f.reviews, reviewID)
	for key := range f.likes {
		if key.reviewID == reviewID {
			delete(f.likes, key)
		}
	}
	f.recalc(r.BookID)
	return nil
}

func (f *fakeRepository) ToggleLike(ctx context.Context, reviewID, userID, revieweeID uuid.UUID) (bool, int, error) {
	key := reviewUserKey{reviewID: reviewID, userID: userID}
	liked := false
	if _, ok := f.likes[key]; ok {
		delete(f.likes, key)
		liked = false
	} else {
		f.likes[key] = true
		liked = true

		if revieweeID != userID {
			if n, ok := f.notifications[key]; ok {
				n.IsRead = false
				n.CreatedAt = time.Now()
			} else {
				f.notifications[key] = &fakeNotification{IsRead: false, CreatedAt: time.Now()}
			}
		}
	}

	count := 0
	for k := range f.likes {
		if k.reviewID == reviewID {
			count++
		}
	}
	if r, ok := f.reviews[reviewID]; ok {
		r.LikesCount = count
	}
	return liked, count, nil
}

func (f *fakeRepository) BookExists(ctx context.Context, bookID uuid.UUID) (bool, error) {
	book, ok := f.books[bookID]
	return ok && (book.Status == "" || book.IsPublished()), nil
}

func setupTestService() (*Service, *fakeRepository) {
	repo := newFakeRepository()
	return NewService(repo), repo
}

func TestCreateReview(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, userID, bookID, 5, "Great book!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if review.Rating != 5 {
		t.Fatalf("expected rating 5, got %d", review.Rating)
	}
	if review.Content != "Great book!" {
		t.Fatalf("expected content preserved, got %q", review.Content)
	}
	if repo.books[bookID].ReviewsCount != 1 {
		t.Fatalf("expected reviews count 1, got %d", repo.books[bookID].ReviewsCount)
	}
	if repo.books[bookID].RatingsCount != 1 {
		t.Fatalf("expected ratings count 1, got %d", repo.books[bookID].RatingsCount)
	}
}

func TestCreateReview_Duplicate(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	if _, err := svc.CreateReview(ctx, userID, bookID, 4, "First"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := svc.CreateReview(ctx, userID, bookID, 5, "Second")
	if !errors.Is(err, ErrAlreadyReviewed) {
		t.Fatalf("expected ErrAlreadyReviewed, got %v", err)
	}
}

func TestCreateReview_InvalidContent(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	cases := []string{"", "   ", string(make([]byte, 10001))}
	for _, content := range cases {
		_, err := svc.CreateReview(ctx, userID, bookID, 3, content)
		if !errors.Is(err, ErrInvalidContent) {
			t.Fatalf("expected ErrInvalidContent for %q, got %v", content, err)
		}
	}
}

func TestCreateReview_InvalidRating(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	cases := []int{0, 6, -1}
	for _, rating := range cases {
		_, err := svc.CreateReview(ctx, userID, bookID, rating, "Valid content")
		if !errors.Is(err, ErrInvalidRating) {
			t.Fatalf("expected ErrInvalidRating for %d, got %v", rating, err)
		}
	}
}

func TestUpdateReview(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, userID, bookID, 3, "Okay")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newRating := 5
	newContent := "Much better"
	updated, err := svc.UpdateReview(ctx, userID, review.ID, &newRating, &newContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Rating != 5 {
		t.Fatalf("expected rating 5, got %d", updated.Rating)
	}
	if updated.Content != "Much better" {
		t.Fatalf("expected updated content, got %q", updated.Content)
	}
}

func TestUpdateReview_NothingToUpdate(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, userID, bookID, 4, "Good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.UpdateReview(ctx, userID, review.ID, nil, nil)
	if !errors.Is(err, ErrNothingToUpdate) {
		t.Fatalf("expected ErrNothingToUpdate, got %v", err)
	}
}

func TestUpdateReview_Forbidden(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, ownerID, bookID, 4, "Good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newContent := "Hacked"
	_, err = svc.UpdateReview(ctx, otherID, review.ID, nil, &newContent)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestDeleteReview(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, userID, bookID, 5, "Great")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.DeleteReview(ctx, userID, review.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.reviews[review.ID]; ok {
		t.Fatal("expected review to be deleted")
	}
	if repo.books[bookID].ReviewsCount != 0 {
		t.Fatalf("expected reviews count 0, got %d", repo.books[bookID].ReviewsCount)
	}
	if repo.books[bookID].RatingsCount != 1 {
		t.Fatalf("expected rating to remain after review deletion, got %d", repo.books[bookID].RatingsCount)
	}
}

func TestDeleteReview_Forbidden(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, ownerID, bookID, 4, "Good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = svc.DeleteReview(ctx, otherID, review.ID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestToggleLike(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, userID, bookID, 5, "Great")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	otherID := uuid.New()
	count, liked, err := svc.ToggleLike(ctx, otherID, review.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !liked {
		t.Fatal("expected liked true")
	}
	if count != 1 {
		t.Fatalf("expected likes count 1, got %d", count)
	}

	count, liked, err = svc.ToggleLike(ctx, otherID, review.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if liked {
		t.Fatal("expected liked false")
	}
	if count != 0 {
		t.Fatalf("expected likes count 0, got %d", count)
	}
}

func TestListReviews(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	for i := 0; i < 5; i++ {
		review := &reviewsDomain.Review{
			ID:        uuid.New(),
			BookID:    bookID,
			UserID:    uuid.New(),
			Rating:    4,
			Content:   "Review",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Hour),
		}
		repo.reviews[review.ID] = review
	}

	result, err := svc.ListReviews(ctx, bookID, &userID, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 5 {
		t.Fatalf("expected total 5, got %d", result.Total)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Page != 1 || result.Limit != 2 {
		t.Fatalf("unexpected pagination: page=%d limit=%d", result.Page, result.Limit)
	}
}

func TestListReviews_MyReview(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	myReview := &reviewsDomain.Review{
		ID:        uuid.New(),
		BookID:    bookID,
		UserID:    userID,
		Rating:    5,
		Content:   "Mine",
		CreatedAt: time.Now(),
	}
	repo.reviews[myReview.ID] = myReview

	result, err := svc.ListReviews(ctx, bookID, &userID, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MyReview == nil {
		t.Fatal("expected my_review")
	}
	if result.MyReview.ID != myReview.ID {
		t.Fatalf("expected my_review id %s, got %s", myReview.ID, result.MyReview.ID)
	}
}

func TestToggleLike_CreatesNotificationForOtherUsersReview(t *testing.T) {
	ctx := context.Background()
	authorID := uuid.New()
	actorID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, authorID, bookID, 5, "Great")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, liked, err := svc.ToggleLike(ctx, actorID, review.ID); err != nil || !liked {
		t.Fatalf("unexpected result: liked=%v err=%v", liked, err)
	}

	key := reviewUserKey{reviewID: review.ID, userID: actorID}
	notification, ok := repo.notifications[key]
	if !ok {
		t.Fatal("expected a notification to be created for the review's author")
	}
	if notification.IsRead {
		t.Fatal("expected a freshly created notification to be unread")
	}
}

func TestToggleLike_SelfLikeDoesNotCreateNotification(t *testing.T) {
	ctx := context.Background()
	authorID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, authorID, bookID, 5, "Great")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, _, err := svc.ToggleLike(ctx, authorID, review.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.notifications) != 0 {
		t.Fatalf("expected no notification for a self-like, got %d", len(repo.notifications))
	}
}

func TestToggleLike_UnlikeDoesNotDeleteExistingNotification(t *testing.T) {
	ctx := context.Background()
	authorID := uuid.New()
	actorID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, authorID, bookID, 5, "Great")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, _, err := svc.ToggleLike(ctx, actorID, review.ID); err != nil { // like
		t.Fatalf("unexpected error: %v", err)
	}
	if _, liked, err := svc.ToggleLike(ctx, actorID, review.ID); err != nil || liked { // unlike
		t.Fatalf("unexpected result: liked=%v err=%v", liked, err)
	}

	key := reviewUserKey{reviewID: review.ID, userID: actorID}
	if _, ok := repo.notifications[key]; !ok {
		t.Fatal("expected the notification to remain after unliking")
	}
}

func TestToggleLike_RepeatedCycleReusesNotificationRow(t *testing.T) {
	ctx := context.Background()
	authorID := uuid.New()
	actorID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	review, err := svc.CreateReview(ctx, authorID, bookID, 5, "Great")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, _, err := svc.ToggleLike(ctx, actorID, review.ID); err != nil { // like
		t.Fatalf("unexpected error: %v", err)
	}

	key := reviewUserKey{reviewID: review.ID, userID: actorID}
	repo.notifications[key].IsRead = true // simulate the recipient having read it

	if _, _, err := svc.ToggleLike(ctx, actorID, review.ID); err != nil { // unlike
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := svc.ToggleLike(ctx, actorID, review.ID); err != nil { // like again
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.notifications) != 1 {
		t.Fatalf("expected the existing notification row to be reused, not duplicated; got %d rows", len(repo.notifications))
	}
	if repo.notifications[key].IsRead {
		t.Fatal("expected the reused notification to be refreshed back to unread")
	}
}

func seedHiddenReview(repo *fakeRepository, userID, bookID uuid.UUID) *reviewsDomain.Review {
	review := &reviewsDomain.Review{
		ID:        uuid.New(),
		BookID:    bookID,
		UserID:    userID,
		Rating:    4,
		Content:   "Hidden text",
		IsHidden:  true,
		CreatedAt: time.Now(),
	}
	repo.reviews[review.ID] = review
	repo.ratings[bookUserKey{bookID: bookID, userID: userID}] = &ratingsDomain.Rating{BookID: bookID, UserID: userID, Value: 4}
	repo.recalc(bookID)
	return review
}

func TestDeleteReview_HiddenByAuthor(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}
	review := seedHiddenReview(repo, userID, bookID)

	if err := svc.DeleteReview(ctx, userID, review.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.reviews[review.ID]; ok {
		t.Fatal("expected hidden review to be deleted")
	}
	if repo.books[bookID].ReviewsCount != 0 {
		t.Fatalf("expected reviews count 0, got %d", repo.books[bookID].ReviewsCount)
	}
	if repo.books[bookID].RatingsCount != 1 {
		t.Fatalf("expected rating to remain, got %d", repo.books[bookID].RatingsCount)
	}
}

func TestDeleteReview_HiddenByOtherUser(t *testing.T) {
	ctx := context.Background()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}
	review := seedHiddenReview(repo, uuid.New(), bookID)

	err := svc.DeleteReview(ctx, uuid.New(), review.ID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if _, ok := repo.reviews[review.ID]; !ok {
		t.Fatal("review must not be deleted")
	}
}

func TestUpdateReview_HiddenByAuthor(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}
	review := seedHiddenReview(repo, userID, bookID)

	newContent := "Edited"
	_, err := svc.UpdateReview(ctx, userID, review.ID, nil, &newContent)
	if !errors.Is(err, ErrReviewHidden) {
		t.Fatalf("expected ErrReviewHidden, got %v", err)
	}
	if repo.reviews[review.ID].Content != "Hidden text" {
		t.Fatalf("review must stay unchanged, got %q", repo.reviews[review.ID].Content)
	}

	_, err = svc.UpdateReview(ctx, uuid.New(), review.ID, nil, &newContent)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for another user, got %v", err)
	}
}

func TestListReviews_HiddenOwnReviewIsMyReviewOnly(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}
	review := seedHiddenReview(repo, userID, bookID)

	result, err := svc.ListReviews(ctx, bookID, &userID, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MyReview == nil || result.MyReview.ID != review.ID || !result.MyReview.IsHidden {
		t.Fatalf("expected hidden my_review, got %+v", result.MyReview)
	}
	if len(result.Items) != 0 {
		t.Fatalf("hidden review must not be in items, got %d", len(result.Items))
	}
}

func TestCreateReview_ContentLimitCountsCharacters(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	if _, err := svc.CreateReview(ctx, userID, bookID, 3, strings.Repeat("я", 10001)); !errors.Is(err, ErrInvalidContent) {
		t.Fatalf("expected ErrInvalidContent for 10001 chars, got %v", err)
	}
	if _, err := svc.CreateReview(ctx, userID, bookID, 3, strings.Repeat("я", 10000)); err != nil {
		t.Fatalf("expected 10000 Cyrillic chars accepted, got %v", err)
	}
}

func TestUpdateReview_ContentLimitCountsCharacters(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}
	review, err := svc.CreateReview(ctx, userID, bookID, 3, "Okay")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tooLong := strings.Repeat("я", 10001)
	if _, err := svc.UpdateReview(ctx, userID, review.ID, nil, &tooLong); !errors.Is(err, ErrInvalidContent) {
		t.Fatalf("expected ErrInvalidContent for 10001 chars, got %v", err)
	}
	ok := strings.Repeat("я", 10000)
	if _, err := svc.UpdateReview(ctx, userID, review.ID, nil, &ok); err != nil {
		t.Fatalf("expected 10000 Cyrillic chars accepted, got %v", err)
	}
}
