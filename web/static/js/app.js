/* ---- Toasts (inline notifications instead of alert()) ---- */

function showToast(message, variant = 'error') {
  const stack = document.getElementById('toast-stack');
  if (!stack) {
    alert(message); // graceful fallback if the stack is missing
    return;
  }
  const toast = document.createElement('div');
  toast.className = `toast${variant === 'info' ? ' toast-info' : ''}`;
  toast.textContent = message;
  stack.appendChild(toast);
  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transition = 'opacity 200ms ease';
    setTimeout(() => toast.remove(), 220);
  }, 3500);
}

/* ---- Reactions: posts + comments ---- */

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

    const likeSpan = bar.querySelector('.like-count');
    const dislikeSpan = bar.querySelector('.dislike-count');
    if (likeSpan && result.likes !== undefined) likeSpan.textContent = result.likes;
    if (dislikeSpan && result.dislikes !== undefined) dislikeSpan.textContent = result.dislikes;

    // Reflect the server's state: highlight the active button, or clear
    // both when the reaction was toggled off.
    bar.querySelectorAll('.react-btn').forEach((b) => b.classList.remove('active'));
    if (result.user_reaction === 'like' || result.user_reaction === 'dislike') {
      const active = bar.querySelector(`.react-btn[data-reaction="${result.user_reaction}"]`);
      if (active) active.classList.add('active');
    }
  } catch (err) {
    showToast(err.message || 'Could not save your reaction. Please try again.');
  } finally {
    btn.disabled = false;
  }
});

/* ---- Form submissions ---- */

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

  const loginForm = e.target.closest('#login-form');
  if (loginForm) {
    e.preventDefault();
    await handleLoginSubmit(loginForm);
    return;
  }

  const registerForm = e.target.closest('#register-form');
  if (registerForm) {
    e.preventDefault();
    await handleRegisterSubmit(registerForm);
    return;
  }

  const logoutForm = e.target.closest('#logout-form');
  if (logoutForm) {
    e.preventDefault();
    try {
      await api.logout();
    } catch (_) {
      // ignore — clear the page either way
    }
    window.location.href = '/';
  }
});

function showFormError(form, message) {
  const errorEl = form.querySelector('.form-error') || document.querySelector('.form-error');
  if (errorEl) {
    errorEl.textContent = message;
    errorEl.hidden = false;
  } else {
    showToast(message);
  }
}

async function handleLoginSubmit(form) {
  const email = form.querySelector('input[name="email"]').value.trim();
  const password = form.querySelector('input[name="password"]').value;

  const submitBtn = form.querySelector('button[type="submit"]');
  submitBtn.disabled = true;
  try {
    await api.login(email, password);
    window.location.href = '/';
  } catch (err) {
    showFormError(form, err.message || 'Could not log in. Please try again.');
  } finally {
    submitBtn.disabled = false;
  }
}

async function handleRegisterSubmit(form) {
  const email = form.querySelector('input[name="email"]').value.trim();
  const username = form.querySelector('input[name="username"]').value.trim();
  const password = form.querySelector('input[name="password"]').value;

  const submitBtn = form.querySelector('button[type="submit"]');
  submitBtn.disabled = true;
  try {
    await api.register(email, username, password);
    window.location.href = '/';
  } catch (err) {
    showFormError(form, err.message || 'Could not register. Please try again.');
  } finally {
    submitBtn.disabled = false;
  }
}

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
    showToast(err.message || 'Could not post your comment. Please try again.');
  } finally {
    submitBtn.disabled = false;
  }
}

async function handleNewPostSubmit(form) {
  const errorEl = document.getElementById('new-post-error');
  errorEl.hidden = true;

  const title = form.querySelector('#post-title').value.trim();
  const body = form.querySelector('#post-body').value.trim();
  const categories = Array.from(
    form.querySelectorAll('input[name="categories"]:checked')
  ).map((input) => input.value);

  if (!title || !body) {
    errorEl.textContent = 'Title and body are required.';
    errorEl.hidden = false;
    return;
  }
  if (categories.length === 0) {
    errorEl.textContent = 'Pick at least one category.';
    errorEl.hidden = false;
    return;
  }

  const submitBtn = form.querySelector('button[type="submit"]');
  submitBtn.disabled = true;

  try {
    const created = await api.createPost(title, body, categories);
    if (created && created.id) {
      window.location.href = `/post/${created.id}`;
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

/* ---- Comments: append newly posted comment ---- */

function appendComment(comment) {
  const list = document.querySelector('.comment-list');
  if (!list) return;

  // Remove the "no comments yet" placeholder if present
  const emptyState = list.querySelector('.empty-state');
  if (emptyState) emptyState.remove();

  const li = document.createElement('li');
  li.className = 'comment';
  li.dataset.commentId = comment.id;

  const when = comment.created_at_human || (comment.created_at ? new Date(comment.created_at).toLocaleString() : 'just now');
  li.innerHTML = `
    <p class="comment-meta">${escapeHtml(comment.user ?? 'You')} · ${escapeHtml(when)}</p>
    <p class="comment-body">${escapeHtml(comment.body)}</p>
    <div class="reaction-bar" data-target-type="comment" data-target-id="${escapeHtml(comment.id)}">
      <button type="button" class="react-btn vote-up" data-reaction="like" aria-label="Upvote comment">
        <span class="arrow">▲</span> <span class="like-count">${comment.likes ?? 0}</span>
      </button>
      <button type="button" class="react-btn vote-down" data-reaction="dislike" aria-label="Downvote comment">
        <span class="arrow">▼</span> <span class="dislike-count">${comment.dislikes ?? 0}</span>
      </button>
    </div>
  `;
  list.appendChild(li);
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = String(str);
  return div.innerHTML;
}

/* ---- Sidebar count badges (best-effort hydration) ---- */

(async function hydrateSidebarCounts() {
  if (!document.querySelector('.count-badge')) return;

  // Category counts come from the public posts list.
  try {
    const posts = await api.request('/api/posts');
    if (Array.isArray(posts)) {
      const allBadge = document.querySelector('.count-badge[data-count="all"]');
      if (allBadge) allBadge.textContent = posts.length;

      const byCategory = {};
      for (const p of posts) {
        for (const c of p.categories || []) {
          byCategory[c] = (byCategory[c] || 0) + 1;
        }
      }
      document.querySelectorAll('.count-badge[data-category-count]').forEach((badge) => {
        const name = badge.dataset.categoryCount;
        badge.textContent = byCategory[name] ?? 0;
      });
    }
  } catch (_) {
    // Counts are decorative — fail silently.
  }

  // Personal counts require auth; skip quietly for guests.
  try {
    const mine = await api.request('/api/posts?filter=my-posts');
    if (Array.isArray(mine)) {
      const badge = document.querySelector('.count-badge[data-count="my-posts"]');
      if (badge) badge.textContent = mine.length;
    }
  } catch (_) { /* guest — ignore */ }

  try {
    const liked = await api.request('/api/posts?filter=my-liked');
    if (Array.isArray(liked)) {
      const badge = document.querySelector('.count-badge[data-count="my-liked"]');
      if (badge) badge.textContent = liked.length;
    }
  } catch (_) { /* guest — ignore */ }
})();
