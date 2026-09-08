
document.addEventListener('click', async (e) => {
  const btn = e.target.closest('.react-btn');
  if (!btn || btn.disabled) return;

  const bar = btn.closest('.reaction-bar');
  const targetType = bar.dataset.targetType; // "post" | "comment"
  const targetId = bar.dataset.targetId;
  const reaction = btn.dataset.reaction; // "like" | "dislike"

  btn.disabled = true;
  try {
    const result =
      targetType === 'post'
        ? await api.reactToPost(targetId, reaction)
        : await api.reactToComment(targetId, reaction);

    // ASSUMPTION: result is { likeCount, dislikeCount }. Adjust the two lines
    // below if the real backend response uses different field names.
    const likeSpan = bar.querySelector('.like-count');
    const dislikeSpan = bar.querySelector('.dislike-count');
    if (likeSpan && result.likeCount !== undefined) likeSpan.textContent = result.likeCount;
    if (dislikeSpan && result.dislikeCount !== undefined) dislikeSpan.textContent = result.dislikeCount;

    bar.querySelectorAll('.react-btn').forEach((b) => b.classList.remove('active'));
    btn.classList.add('active');
  } catch (err) {
    alert(err.message || 'Could not save your reaction. Please try again.');
  } finally {
    btn.disabled = false;
  }
});

document.addEventListener('submit', async (e) => {
  const form = e.target.closest('.comment-form');
  if (!form) return;
  e.preventDefault();

  const postId = form.dataset.postId;
  const textarea = form.querySelector('textarea[name="body"]');
  const body = textarea.value.trim();
  if (!body) return;

  const submitBtn = form.querySelector('button[type="submit"]');
  submitBtn.disabled = true;

  try {
    const comment = await api.addComment(postId, body);
    appendComment(comment);
    textarea.value = '';
  } catch (err) {
    alert(err.message || 'Could not post your comment. Please try again.');
  } finally {
    submitBtn.disabled = false;
  }
});

function appendComment(comment) {
  const list = document.querySelector('.comment-list');
  if (!list) return;

  // Remove the "no comments yet" placeholder if present
  const emptyState = list.querySelector('.empty-state');
  if (emptyState) emptyState.remove();

  const li = document.createElement('li');
  li.className = 'comment';
  li.dataset.commentId = comment.id;

  // ASSUMPTION: comment.author.username and comment.createdAt exist —
  // adjust if internal/comments returns different field names.
  li.innerHTML = `
    <p class="comment-meta">${escapeHtml(comment.author?.username ?? 'You')} · ${escapeHtml(comment.createdAt ?? 'just now')}</p>
    <p class="comment-body">${escapeHtml(comment.body)}</p>
    <div class="reaction-bar" data-target-type="comment" data-target-id="${comment.id}">
      <button class="react-btn" data-reaction="like">▲ <span class="like-count">0</span></button>
      <button class="react-btn" data-reaction="dislike">▼ <span class="dislike-count">0</span></button>
    </div>
  `;
  list.appendChild(li);
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}