

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
  const commentForm = e.target.closest('.comment-form');
  if (commentForm) {
    e.preventDefault();
    await handleCommentSubmit(commentForm);
    return;
  }

  const newPostForm = e.target.closest('#new-post-form');
  if (newPostForm) {
    e.preventDefault();
    await handleNewPostSubmit(newPostForm);
    return;
  }
});

async function handleCommentSubmit(form) {
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
}

async function handleNewPostSubmit(form) {
  const errorEl = document.getElementById('new-post-error');
  errorEl.hidden = true;

  const title = form.querySelector('#post-title').value.trim();
  const body = form.querySelector('#post-body').value.trim();
  const categoryIds = Array.from(
    form.querySelectorAll('input[name="categoryIds"]:checked')
  ).map((input) => input.value);

  if (!title || !body) {
    errorEl.textContent = 'Title and body are required.';
    errorEl.hidden = false;
    return;
  }

  const submitBtn = form.querySelector('button[type="submit"]');
  submitBtn.disabled = true;

  try {
    // ASSUMPTION: response includes the new post's id so we can redirect there.
    // Adjust the field name below once confirmed against internal/posts.Create.
    const created = await api.createPost(title, body, categoryIds);
    if (created && created.id) {
      window.location.href = `/posts/${created.id}`;
    } else {
      window.location.href = '/';
    }
  } catch (err) {
    errorEl.textContent = err.message || 'Could not publish your post. Please try again.';
    errorEl.hidden = false;
  } finally {
    submitBtn.disabled = false;
  }
}

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