const USER_KEY = "gophprofile_user_id";

function currentUser() {
  return localStorage.getItem(USER_KEY) || "demo";
}

function setUser(id) {
  localStorage.setItem(USER_KEY, id);
}

function galleryPath(userId) {
  return `/web/gallery/${encodeURIComponent(userId)}`;
}

function initNav() {
  const link = document.getElementById("gallery-link");
  if (link) link.href = galleryPath(currentUser());
}

function initUpload() {
  const form = document.getElementById("upload-form");
  if (!form) return;

  const userInput = document.getElementById("user-id");
  const fileInput = document.getElementById("file-input");
  const preview = document.getElementById("preview");
  const status = document.getElementById("status");
  const progress = document.getElementById("progress");
  const bar = progress.querySelector("div");
  const submit = document.getElementById("submit-btn");

  userInput.value = currentUser();
  userInput.addEventListener("change", () => setUser(userInput.value.trim() || "demo"));

  fileInput.addEventListener("change", () => {
    const file = fileInput.files[0];
    if (!file) return;
    preview.src = URL.createObjectURL(file);
    preview.hidden = false;
  });

  ["dragenter", "dragover"].forEach((ev) => {
    form.addEventListener(ev, (e) => {
      e.preventDefault();
      form.classList.add("drag");
    });
  });
  ["dragleave", "drop"].forEach((ev) => {
    form.addEventListener(ev, (e) => {
      e.preventDefault();
      form.classList.remove("drag");
    });
  });
  form.addEventListener("drop", (e) => {
    if (e.dataTransfer.files[0]) {
      fileInput.files = e.dataTransfer.files;
      fileInput.dispatchEvent(new Event("change"));
    }
  });

  submit.addEventListener("click", () => {
    const userId = userInput.value.trim();
    const file = fileInput.files[0];
    if (!userId || !file) {
      status.textContent = "Укажите User ID и выберите файл";
      return;
    }
    setUser(userId);

    const body = new FormData();
    body.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/v1/avatars");
    xhr.setRequestHeader("X-User-ID", userId);
    progress.hidden = false;
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) bar.style.width = `${Math.round((e.loaded / e.total) * 100)}%`;
    };
    xhr.onload = () => {
      if (xhr.status === 201) {
        status.textContent = "Загружено, обработка запущена";
        window.location.href = galleryPath(userId);
        return;
      }
      try {
        const payload = JSON.parse(xhr.responseText);
        status.textContent = payload.error || "Ошибка загрузки";
      } catch {
        status.textContent = "Ошибка загрузки";
      }
    };
    xhr.onerror = () => {
      status.textContent = "Сеть недоступна";
    };
    xhr.send(body);
  });
}

function initGallery() {
  const grid = document.getElementById("grid");
  if (!grid) return;

  const parts = window.location.pathname.split("/");
  const userId = decodeURIComponent(parts[parts.length - 1] || currentUser());
  setUser(userId);
  document.getElementById("user-label").textContent = `user_id: ${userId}`;

  fetch(`/api/v1/users/${encodeURIComponent(userId)}/avatars`)
    .then((r) => r.json())
    .then((items) => {
      if (!Array.isArray(items) || items.length === 0) {
        document.getElementById("empty").hidden = false;
        return;
      }
      grid.innerHTML = items.map((item) => {
        const id = encodeURIComponent(item.id);
        const src = `/api/v1/avatars/${id}?size=300x300`;
        return `
          <article class="tile" data-id="${id}">
            <img src="${src}" alt="${escapeHtml(item.file_name)}">
            <footer>
              <span class="badge">${escapeHtml(item.status)}</span>
              <button class="ghost" data-delete="${id}">Удалить</button>
            </footer>
          </article>`;
      }).join("");

      grid.querySelectorAll("[data-delete]").forEach((btn) => {
        btn.addEventListener("click", async () => {
          const id = btn.getAttribute("data-delete");
          const res = await fetch(`/api/v1/avatars/${id}`, {
            method: "DELETE",
            headers: { "X-User-ID": userId },
          });
          if (res.status === 204) {
            btn.closest(".tile").remove();
            if (!grid.children.length) document.getElementById("empty").hidden = false;
          }
        });
      });
    });
}

function escapeHtml(value) {
  return String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

initNav();
initUpload();
initGallery();
