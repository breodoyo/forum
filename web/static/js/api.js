
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
        if (errBody && errBody.error) message = errBody.error;
      } catch (_) {
        // response wasn't JSON — keep the generic message
      }
      throw new Error(message);
    }

    if (res.status === 204) return null;
    return res.json();
  },

  // ---- Auth ----
  login(email, password) {
    return api.request('/api/login', { method: 'POST', body: { email, password } });
  },

  register(email, username, password) {
    return api.request('/api/register', { method: 'POST', body: { email, username, password } });
  },

  logout() {
    return api.request('/api/logout', { method: 'POST' });
  },

  // ---- Reactions ----
  // Both endpoints return { likes, dislikes, user_reaction } where
  // user_reaction is "like", "dislike", or "" when the reaction was toggled off.
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
  // Returns the created comment: { id, post_id, user, body, created_at, likes, dislikes }.
  addComment(postId, body) {
    return api.request(`/api/posts/${postId}/comments`, {
      method: 'POST',
      body: { body },
    });
  },

  // ---- Posts ----
  // Expects { title, body, categories } where categories is a list of
  // category names; returns { id } on success.
  createPost(title, body, categories) {
    return api.request('/api/posts', {
      method: 'POST',
      body: { title, body, categories },
    });
  },
};
