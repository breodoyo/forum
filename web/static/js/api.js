

const api = {
  async request(path, { method = 'GET', body } = {}) {
    const res = await fetch(path, {
      method,
      credentials: 'same-origin',
      headers: body ? { 'Content-Type': 'application/json' } : {},
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!res.ok) {
      let message = `Request failed (${res.status})`;
      try {
        const errBody = await res.json();
        // ASSUMPTION: error responses look like { error: "message" } —
        // confirm the real shape against internal/httputil.Error and adjust if needed.
        if (errBody && errBody.error) message = errBody.error;
      } catch (_) {
        // response wasn't JSON — keep the generic message
      }
      throw new Error(message);
    }

    if (res.status === 204) return null;
    return res.json();
  },

  // ---- Reactions ----
  // ASSUMPTION: both endpoints return the updated counts, e.g. { likeCount, dislikeCount }.
  // Confirm against internal/reactions and adjust the field names in app.js if different.
  reactToPost(postId, reaction) {
    return api.request(`/api/posts/${postId}/react`, {
      method: 'POST',
      body: { type: reaction }, // "like" | "dislike"
    });
  },

  reactToComment(commentId, reaction) {
    return api.request(`/api/comments/${commentId}/react`, {
      method: 'POST',
      body: { type: reaction },
    });
  },

  // ---- Comments ----
  // ASSUMPTION: returns the created comment, e.g.
  // { id, body, createdAt, author: { username }, likeCount, dislikeCount }.
  // Confirm against internal/comments and adjust renderComment() in app.js if different.
  addComment(postId, body) {
    return api.request(`/api/posts/${postId}/comments`, {
      method: 'POST',
      body: { body },
    });
  },

  // ---- Posts ----
  // ASSUMPTION: expects { title, body, categoryIds } and returns the created post
  // (at minimum an { id } so we know where to redirect). Confirm the request/response
  // shape against internal/posts.Create and adjust here if different.
  createPost(title, body, categoryIds) {
    return api.request('/api/posts', {
      method: 'POST',
      body: { title, body, categoryIds },
    });
  },
};