/** @format */

document.documentElement.classList.add("js");

const STORAGE_KEYS = {
  botToken: "telerealm.botToken",
  chatId: "telerealm.chatId",
  uploadedFiles: "telerealm.uploadedFiles",
};

const PAGE_LOADER_ID = "pageLoader";
const FLOATING_UPLOAD_PROGRESS_ID = "floatingUploadProgress";
const MULTIPART_THRESHOLD_BYTES = 45 * 1024 * 1024;
const MULTIPART_CHUNK_SIZE_BYTES = 8 * 1024 * 1024;
const MULTIPART_INIT_PATHS = ["/multipart/init", "/api/multipart/init"];

let selectedFiles = [];
let botToken = "";
let chatId = "";

// Load saved configuration
window.onload = function () {
  hydrateSavedConfig();

  initPageLoader();
  loadUploadedFiles();
  initRevealAnimation();
  initSetupGuide();
  initLocalNotice();
  initSetupVideo();
  initUploadUI();
  initLocalActions();
  initFloatingUploadProgress();
};

function initPageLoader() {
  ensurePageLoader();

  window.addEventListener("pageshow", () => {
    hidePageLoader();
  });

  document.addEventListener("click", (event) => {
    const target = event.target;
    if (!(target instanceof Element)) return;

    const link = target.closest("a[href]");
    if (!(link instanceof HTMLAnchorElement)) return;

    if (
      event.defaultPrevented ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey ||
      event.button !== 0 ||
      link.target === "_blank" ||
      link.hasAttribute("download")
    ) {
      return;
    }

    const href = link.getAttribute("href") || "";
    if (
      !href ||
      href.startsWith("#") ||
      href.startsWith("mailto:") ||
      href.startsWith("tel:") ||
      href.startsWith("javascript:")
    ) {
      return;
    }

    let nextURL;
    try {
      nextURL = new URL(link.href, window.location.href);
    } catch (_error) {
      return;
    }

    const samePageHashOnly =
      nextURL.origin === window.location.origin &&
      nextURL.pathname === window.location.pathname &&
      nextURL.search === window.location.search &&
      nextURL.hash;

    if (samePageHashOnly) return;

    if (nextURL.origin === window.location.origin) {
      showPageLoader("Opening page...");
    }
  });
}

function ensurePageLoader() {
  if (document.getElementById(PAGE_LOADER_ID)) return;

  const loader = document.createElement("div");
  loader.id = PAGE_LOADER_ID;
  loader.className = "page-loader";
  loader.hidden = true;
  loader.setAttribute("aria-hidden", "true");
  loader.innerHTML = `
    <div class="page-loader__backdrop"></div>
    <div class="page-loader__panel" role="status" aria-live="polite">
      <span class="page-loader__spinner" aria-hidden="true"></span>
      <div>
        <strong>TeleRealm</strong>
        <p id="pageLoaderMessage">Loading page...</p>
      </div>
    </div>
  `;

  document.body.appendChild(loader);
}

function showPageLoader(message = "Loading page...") {
  const loader = document.getElementById(PAGE_LOADER_ID);
  const messageNode = document.getElementById("pageLoaderMessage");

  if (!loader) return;

  if (messageNode) {
    messageNode.textContent = message;
  }

  loader.hidden = false;
  loader.setAttribute("aria-hidden", "false");
  document.body.classList.add("is-page-loading");
}

function hidePageLoader() {
  const loader = document.getElementById(PAGE_LOADER_ID);
  if (!loader) return;

  loader.hidden = true;
  loader.setAttribute("aria-hidden", "true");
  document.body.classList.remove("is-page-loading");
}

function initFloatingUploadProgress() {
  ensureFloatingUploadProgress();
  resetFloatingUploadProgress();
}

function ensureFloatingUploadProgress() {
  if (document.getElementById(FLOATING_UPLOAD_PROGRESS_ID)) return;

  const widget = document.createElement("aside");
  widget.id = FLOATING_UPLOAD_PROGRESS_ID;
  widget.className = "floating-upload-progress";
  widget.hidden = true;
  widget.setAttribute("aria-live", "polite");
  widget.innerHTML = `
    <div class="floating-upload-progress__header">
      <strong>Upload Progress</strong>
      <span id="floatingUploadPercent">0%</span>
    </div>
    <p id="floatingUploadFile">Waiting for upload...</p>
    <div class="floating-upload-progress__bar">
      <div id="floatingUploadBar" class="floating-upload-progress__fill"></div>
    </div>
    <small id="floatingUploadMeta">0 / 0 files completed</small>
  `;

  document.body.appendChild(widget);
}

function updateFloatingUploadProgress({
  visible = true,
  fileName = "Waiting for upload...",
  percent = 0,
  completed = 0,
  total = 0,
}) {
  const widget = document.getElementById(FLOATING_UPLOAD_PROGRESS_ID);
  const percentNode = document.getElementById("floatingUploadPercent");
  const fileNode = document.getElementById("floatingUploadFile");
  const barNode = document.getElementById("floatingUploadBar");
  const metaNode = document.getElementById("floatingUploadMeta");

  if (!widget || !percentNode || !fileNode || !barNode || !metaNode) return;

  const safePercent = Math.max(0, Math.min(100, Math.round(percent)));

  widget.hidden = !visible;
  percentNode.textContent = `${safePercent}%`;
  fileNode.textContent = fileName;
  barNode.style.width = `${safePercent}%`;
  metaNode.textContent = `${completed} / ${total} files completed`;
}

function resetFloatingUploadProgress() {
  updateFloatingUploadProgress({
    visible: false,
    fileName: "Waiting for upload...",
    percent: 0,
    completed: 0,
    total: 0,
  });
}

function syncConfigFromInputs(persist = false) {
  const botTokenInput = document.getElementById("botToken");
  const chatIdInput = document.getElementById("chatId");

  if (!botTokenInput || !chatIdInput) return;

  botToken = botTokenInput.value.trim();
  chatId = chatIdInput.value.trim();

  if (persist && botToken && chatId) {
    localStorage.setItem(STORAGE_KEYS.botToken, botToken);
    localStorage.setItem(STORAGE_KEYS.chatId, chatId);
  }
}

function initSetupVideo() {
  const videoFab = document.getElementById("setupVideoFab");
  const videoModal = document.getElementById("setupVideoModal");
  const videoClose = document.getElementById("setupVideoClose");
  const videoFrame = document.getElementById("setupVideoFrame");
  const videoPlaceholder = document.getElementById("setupVideoPlaceholder");
  const inlineVideoFrame = document.getElementById("setupVideoInlineFrame");
  const inlineVideoPlaceholder = document.getElementById(
    "setupVideoInlinePlaceholder",
  );

  if (
    !videoFab ||
    !videoModal ||
    !videoClose ||
    !videoFrame ||
    !videoPlaceholder
  ) {
    return;
  }

  const videoURL = videoFab.dataset.videoUrl?.trim() || "";

  if (inlineVideoFrame && inlineVideoPlaceholder) {
    if (videoURL) {
      inlineVideoPlaceholder.hidden = true;
      inlineVideoFrame.hidden = false;
      inlineVideoFrame.src = videoURL;
    } else {
      inlineVideoFrame.hidden = true;
      inlineVideoPlaceholder.hidden = false;
    }
  }

  const closeVideo = () => {
    videoModal.hidden = true;
    document.body.style.overflow = "";
    if (videoFrame instanceof HTMLIFrameElement) {
      videoFrame.src = "";
    }
  };

  const openVideo = () => {
    videoModal.hidden = false;
    document.body.style.overflow = "hidden";

    if (videoURL) {
      videoPlaceholder.hidden = true;
      videoFrame.hidden = false;
      videoFrame.src = videoURL;
      return;
    }

    videoFrame.hidden = true;
    videoPlaceholder.hidden = false;
  };

  videoFab.addEventListener("click", openVideo);
  videoClose.addEventListener("click", closeVideo);

  videoModal.addEventListener("click", (event) => {
    const target = event.target;
    if (target instanceof HTMLElement && target.dataset.closeVideo === "true") {
      closeVideo();
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !videoModal.hidden) {
      closeVideo();
    }
  });
}

function hydrateSavedConfig() {
  botToken =
    localStorage.getItem(STORAGE_KEYS.botToken) ||
    localStorage.getItem("botToken") ||
    "";
  chatId =
    localStorage.getItem(STORAGE_KEYS.chatId) ||
    localStorage.getItem("chatId") ||
    "";

  if (botToken) localStorage.setItem(STORAGE_KEYS.botToken, botToken);
  if (chatId) localStorage.setItem(STORAGE_KEYS.chatId, chatId);

  const botTokenInput = document.getElementById("botToken");
  const chatIdInput = document.getElementById("chatId");

  if (botToken && botTokenInput) botTokenInput.value = botToken;
  if (chatId && chatIdInput) chatIdInput.value = chatId;
}

function initUploadUI() {
  const dropZone = document.getElementById("dropZone");
  const fileInput = document.getElementById("fileInput");

  if (!dropZone || !fileInput) return;

  dropZone.addEventListener("click", (event) => {
    const target = event.target;
    if (!(target instanceof Element)) {
      fileInput.click();
      return;
    }

    if (target.closest("label, input, button, a")) {
      return;
    }

    fileInput.click();
  });

  dropZone.addEventListener("dragover", (e) => {
    e.preventDefault();
    dropZone.classList.add("drag-over");
  });

  dropZone.addEventListener("dragleave", () => {
    dropZone.classList.remove("drag-over");
  });

  dropZone.addEventListener("drop", (e) => {
    e.preventDefault();
    dropZone.classList.remove("drag-over");

    const files = Array.from(e.dataTransfer.files);
    addFiles(files);
  });

  fileInput.addEventListener("change", (e) => {
    const files = Array.from(e.target.files);
    addFiles(files);
    fileInput.value = "";
  });

  updateUploadButton();
}

function initLocalActions() {
  const clearLocalDataBtn = document.getElementById("clearLocalDataBtn");
  if (!clearLocalDataBtn) return;

  clearLocalDataBtn.addEventListener("click", () => {
    localStorage.removeItem(STORAGE_KEYS.botToken);
    localStorage.removeItem(STORAGE_KEYS.chatId);
    localStorage.removeItem(STORAGE_KEYS.uploadedFiles);
    localStorage.removeItem("botToken");
    localStorage.removeItem("chatId");
    localStorage.removeItem("uploadedFiles");
    selectedFiles = [];
    botToken = "";
    chatId = "";

    const botTokenInput = document.getElementById("botToken");
    const chatIdInput = document.getElementById("chatId");
    if (botTokenInput) botTokenInput.value = "";
    if (chatIdInput) chatIdInput.value = "";

    renderSelectedFiles();
    updateUploadButton();
    loadUploadedFiles();
    showNotification("Local data cleared", "success");
  });
}

function initSetupGuide() {
  const guideFab = document.getElementById("guideFab");
  const guideModal = document.getElementById("guideModal");
  const guideClose = document.getElementById("guideClose");

  if (!guideFab || !guideModal || !guideClose) return;

  const closeGuide = () => {
    guideModal.hidden = true;
    document.body.style.overflow = "";
  };

  const openGuide = () => {
    guideModal.hidden = false;
    document.body.style.overflow = "hidden";
  };

  guideFab.addEventListener("click", openGuide);
  guideClose.addEventListener("click", closeGuide);

  guideModal.addEventListener("click", (event) => {
    const target = event.target;
    if (target instanceof HTMLElement && target.dataset.closeGuide === "true") {
      closeGuide();
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !guideModal.hidden) {
      closeGuide();
    }
  });
}

function initLocalNotice() {
  const noticeFab = document.getElementById("localNoticeFab");
  const noticeModal = document.getElementById("localNoticeModal");
  const noticeClose = document.getElementById("localNoticeClose");

  if (!noticeFab || !noticeModal || !noticeClose) return;

  const closeNotice = () => {
    noticeModal.hidden = true;
    document.body.style.overflow = "";
  };

  const openNotice = () => {
    noticeModal.hidden = false;
    document.body.style.overflow = "hidden";
  };

  noticeFab.addEventListener("click", openNotice);
  noticeClose.addEventListener("click", closeNotice);

  noticeModal.addEventListener("click", (event) => {
    const target = event.target;
    if (
      target instanceof HTMLElement &&
      target.dataset.closeStorage === "true"
    ) {
      closeNotice();
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !noticeModal.hidden) {
      closeNotice();
    }
  });
}

// Save configuration
function saveConfig() {
  syncConfigFromInputs(true);

  if (!botToken || !chatId) {
    showNotification("Please enter both Bot Token and Chat ID", "error");
    return;
  }

  showNotification("Configuration saved successfully!", "success");
}

// Add files to selection
function addFiles(files) {
  files.forEach((file) => {
    if (
      !selectedFiles.some((f) => f.name === file.name && f.size === file.size)
    ) {
      selectedFiles.push(file);
    }
  });

  renderSelectedFiles();
  updateUploadButton();
}

// Render selected files
function renderSelectedFiles() {
  const container = document.getElementById("selectedFiles");
  if (!container) return;

  if (selectedFiles.length === 0) {
    container.innerHTML = "";
    return;
  }

  container.innerHTML = selectedFiles
    .map(
      (file, index) => `
        <div class="file-item">
            <div class="file-info">
                <div class="file-icon">${getFileExtension(file.name)}</div>
                <div class="file-details">
                    <div class="file-name">${file.name}</div>
                    <div class="file-size">${formatFileSize(file.size)}</div>
                </div>
            </div>
            <button class="btn-remove" onclick="removeFile(${index})">Remove</button>
        </div>
    `,
    )
    .join("");
}

// Remove file from selection
function removeFile(index) {
  selectedFiles.splice(index, 1);
  renderSelectedFiles();
  updateUploadButton();
}

// Update upload button state
function updateUploadButton() {
  const uploadBtn = document.getElementById("uploadBtn");
  if (!uploadBtn) return;
  uploadBtn.disabled = selectedFiles.length === 0;
}

// Upload files
async function uploadFiles() {
  syncConfigFromInputs();

  if (!botToken || !chatId) {
    showNotification("Please configure Bot Token and Chat ID first", "error");
    return;
  }

  if (selectedFiles.length === 0) {
    showNotification("Please select files to upload", "error");
    return;
  }

  const uploadBtn = document.getElementById("uploadBtn");
  const progressContainer = document.getElementById("uploadProgress");

  if (!uploadBtn || !progressContainer) return;

  uploadBtn.disabled = true;
  uploadBtn.textContent = "Uploading...";
  progressContainer.innerHTML = "";

  const totalFiles = selectedFiles.length;
  let completedFiles = 0;

  updateFloatingUploadProgress({
    visible: true,
    fileName: "Preparing uploads...",
    percent: 0,
    completed: completedFiles,
    total: totalFiles,
  });

  for (let i = 0; i < selectedFiles.length; i++) {
    const file = selectedFiles[i];

    // Create progress item
    const progressItem = document.createElement("div");
    progressItem.className = "progress-item";
    progressItem.innerHTML = `
            <div>${file.name}</div>
            <div class="progress-bar">
                <div class="progress-fill" style="width: 0%"></div>
            </div>
        `;
    progressContainer.appendChild(progressItem);

    try {
      const result = await uploadFile(file, progressItem, (filePercent) => {
        const overallPercent = ((i + filePercent / 100) / totalFiles) * 100;

        updateFloatingUploadProgress({
          visible: true,
          fileName: `Uploading ${file.name}`,
          percent: overallPercent,
          completed: completedFiles,
          total: totalFiles,
        });
      });
      const payload = result.data || {};

      // Save to localStorage
      saveUploadedFile({
        name: file.name,
        size: file.size,
        secure_url: payload.secure_url,
        fileId: payload.id,
        timestamp: new Date().toISOString(),
      });

      // Update progress to 100%
      progressItem.querySelector(".progress-fill").style.width = "100%";
      completedFiles += 1;
      updateFloatingUploadProgress({
        visible: true,
        fileName: `${file.name} uploaded`,
        percent: (completedFiles / totalFiles) * 100,
        completed: completedFiles,
        total: totalFiles,
      });
    } catch (error) {
      progressItem.innerHTML += `<div class="error-message">Failed: ${error.message}</div>`;
      updateFloatingUploadProgress({
        visible: true,
        fileName: `Failed: ${file.name}`,
        percent: (completedFiles / totalFiles) * 100,
        completed: completedFiles,
        total: totalFiles,
      });
    }
  }

  // Reset
  selectedFiles = [];
  renderSelectedFiles();
  uploadBtn.disabled = false;
  uploadBtn.textContent = "Upload Files";

  // Reload uploaded files list
  loadUploadedFiles();

  setTimeout(() => {
    resetFloatingUploadProgress();
  }, 1800);

  showNotification("Upload completed!", "success");
}

// Upload single file
async function uploadFile(file, progressItem, onProgress) {
  const progressFill = progressItem.querySelector(".progress-fill");

  if (file.size > MULTIPART_THRESHOLD_BYTES) {
    return uploadFileMultipart(file, progressFill, onProgress);
  }

  return uploadFileDirect(file, progressFill, onProgress);
}

async function uploadFileDirect(file, progressFill, onProgress) {
  return await new Promise((resolve, reject) => {
    const formData = new FormData();
    formData.append("chat_id", chatId);
    formData.append("document", file);

    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/send");
    xhr.setRequestHeader("Authorization", `Bearer ${botToken}`);
    xhr.responseType = "json";

    xhr.upload.addEventListener("progress", (event) => {
      if (!event.lengthComputable) return;

      const percent = (event.loaded / event.total) * 100;
      if (progressFill) {
        progressFill.style.width = `${percent}%`;
      }

      if (typeof onProgress === "function") {
        onProgress(percent);
      }
    });

    xhr.addEventListener("load", () => {
      const payload = xhr.response || safeParseJSON(xhr.responseText);

      if (xhr.status >= 200 && xhr.status < 300) {
        if (progressFill) {
          progressFill.style.width = "100%";
        }

        if (typeof onProgress === "function") {
          onProgress(100);
        }

        resolve(payload);
        return;
      }

      reject(new Error(payload?.message || payload?.error || "Upload failed"));
    });

    xhr.addEventListener("error", () => {
      reject(new Error("Network error while uploading file"));
    });

    xhr.addEventListener("abort", () => {
      reject(new Error("Upload aborted"));
    });

    xhr.send(formData);
  });
}

async function uploadFileMultipart(file, progressFill, onProgress) {
  const initRequestBody = JSON.stringify({
    file_name: file.name,
    file_size: file.size,
    part_size: MULTIPART_CHUNK_SIZE_BYTES,
    content_type: file.type || "application/octet-stream",
    chat_id: chatId,
  });

  const { response: initResponse, path: initPath } = await fetchWith404Fallback(
    MULTIPART_INIT_PATHS,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${botToken}`,
        "Content-Type": "application/json",
      },
      body: initRequestBody,
    },
  );

  const initPayload = await safeReadJSON(initResponse);
  if (!initResponse.ok) {
    throw new Error(
      buildHttpErrorMessage(
        initResponse,
        initPayload,
        "Failed to initialize multipart upload",
      ),
    );
  }

  const session = initPayload?.data || {};
  const uploadID = session.upload_id;
  const totalParts = Number(session.total_parts || 0);
  const multipartPrefix = initPath.startsWith("/api/") ? "/api" : "";

  if (!uploadID || totalParts <= 0) {
    throw new Error("Invalid multipart session response");
  }

  for (let partNumber = 1; partNumber <= totalParts; partNumber++) {
    const start = (partNumber - 1) * MULTIPART_CHUNK_SIZE_BYTES;
    const end = Math.min(start + MULTIPART_CHUNK_SIZE_BYTES, file.size);
    const chunk = file.slice(start, end);

    await uploadMultipartChunk(
      `${multipartPrefix}/multipart/${encodeURIComponent(uploadID)}/part?part_number=${partNumber}`,
      chunk,
      (partPercent) => {
        const uploadPercent =
          ((partNumber - 1 + partPercent / 100) / totalParts) * 100;

        if (progressFill) {
          progressFill.style.width = `${uploadPercent}%`;
        }

        if (typeof onProgress === "function") {
          onProgress(uploadPercent);
        }
      },
    );
  }

  const completeResponse = await fetch(
    `${multipartPrefix}/multipart/${encodeURIComponent(uploadID)}/complete`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${botToken}`,
      },
    },
  );

  const completePayload = await safeReadJSON(completeResponse);
  if (!completeResponse.ok) {
    throw new Error(
      buildHttpErrorMessage(
        completeResponse,
        completePayload,
        "Failed to complete multipart upload",
      ),
    );
  }

  if (progressFill) {
    progressFill.style.width = "100%";
  }
  if (typeof onProgress === "function") {
    onProgress(100);
  }

  return completePayload;
}

async function uploadMultipartChunk(chunkURL, chunk, onProgress) {
  return await new Promise((resolve, reject) => {
    const formData = new FormData();
    formData.append("chunk", chunk, "part");

    const xhr = new XMLHttpRequest();
    xhr.open("POST", chunkURL);
    xhr.setRequestHeader("Authorization", `Bearer ${botToken}`);
    xhr.responseType = "json";

    xhr.upload.addEventListener("progress", (event) => {
      if (!event.lengthComputable) return;
      const percent = (event.loaded / event.total) * 100;
      if (typeof onProgress === "function") {
        onProgress(percent);
      }
    });

    xhr.addEventListener("load", () => {
      const payload = xhr.response || safeParseJSON(xhr.responseText);

      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(payload);
        return;
      }

      reject(
        new Error(buildXhrErrorMessage(xhr, payload, "Chunk upload failed")),
      );
    });

    xhr.addEventListener("error", () => {
      reject(new Error("Network error while uploading chunk"));
    });

    xhr.addEventListener("abort", () => {
      reject(new Error("Chunk upload aborted"));
    });

    xhr.send(formData);
  });
}

async function fetchWith404Fallback(paths, options) {
  let lastResponse = null;

  for (const path of paths) {
    const response = await fetch(path, options);
    if (response.status !== 404) {
      return { response, path };
    }
    lastResponse = response;
  }

  return { response: lastResponse, path: paths[0] };
}

async function safeReadJSON(response) {
  if (!response) return null;

  try {
    return await response.json();
  } catch (_error) {
    return null;
  }
}

function buildHttpErrorMessage(response, payload, fallbackMessage) {
  const status = response?.status;
  const statusText = response?.statusText;
  const detail = payload?.message || payload?.error;

  if (detail && status) {
    return `${fallbackMessage} (${status} ${statusText || ""}): ${detail}`.trim();
  }

  if (detail) {
    return `${fallbackMessage}: ${detail}`;
  }

  if (status) {
    return `${fallbackMessage} (${status} ${statusText || ""})`.trim();
  }

  return fallbackMessage;
}

function buildXhrErrorMessage(xhr, payload, fallbackMessage) {
  const status = xhr?.status;
  const statusText = xhr?.statusText;
  const detail = payload?.message || payload?.error;

  if (detail && status) {
    return `${fallbackMessage} (${status} ${statusText || ""}): ${detail}`.trim();
  }

  if (detail) {
    return `${fallbackMessage}: ${detail}`;
  }

  if (status) {
    return `${fallbackMessage} (${status} ${statusText || ""})`.trim();
  }

  return fallbackMessage;
}

function safeParseJSON(value) {
  if (!value || typeof value !== "string") {
    return null;
  }

  try {
    return JSON.parse(value);
  } catch (_error) {
    return null;
  }
}

// Save uploaded file to localStorage
function saveUploadedFile(fileData) {
  let uploadedFiles = JSON.parse(
    localStorage.getItem(STORAGE_KEYS.uploadedFiles) ||
      localStorage.getItem("uploadedFiles") ||
      "[]",
  );
  uploadedFiles.unshift(fileData);

  // Keep only last 50 files
  if (uploadedFiles.length > 50) {
    uploadedFiles = uploadedFiles.slice(0, 50);
  }

  localStorage.setItem(
    STORAGE_KEYS.uploadedFiles,
    JSON.stringify(uploadedFiles),
  );
}

// Load uploaded files
function loadUploadedFiles() {
  const uploadedFiles = JSON.parse(
    localStorage.getItem(STORAGE_KEYS.uploadedFiles) ||
      localStorage.getItem("uploadedFiles") ||
      "[]",
  );
  const container = document.getElementById("filesList");
  if (!container) return;

  if (uploadedFiles.length === 0) {
    container.innerHTML =
      '<div class="empty-state">No files uploaded yet</div>';
    return;
  }

  container.innerHTML = uploadedFiles
    .map((file, index) => {
      const preview =
        isImageFile(file.name) ?
          `<a class="uploaded-preview" href="${file.secure_url}" target="_blank" rel="noreferrer"><img src="${file.secure_url}" alt="${file.name}"></a>`
        : `<div class="uploaded-preview uploaded-preview--file"><span>${getFileExtension(file.name)}</span></div>`;

      return `
        <div class="uploaded-file-item">
            ${preview}
            <div class="uploaded-file-header">
                <span class="file-name-uploaded">${file.name}</span>
                <span class="upload-time">${formatDate(file.timestamp)}</span>
            </div>
            <div class="file-size">${formatFileSize(file.size)}</div>
            <div class="file-link">
                <input type="text" class="link-input" value="${file.secure_url}" readonly id="link-${index}">
          <button class="btn-copy" onclick="copyLink(${index}, this)">Copy</button>
                <button class="btn-download" onclick="window.open('${file.secure_url}', '_blank')">Download</button>
            </div>
        </div>
          `;
    })
    .join("");
}

// Copy link to clipboard
async function copyLink(index, button) {
  const input = document.getElementById(`link-${index}`);
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(input.value);
    } else {
      input.select();
      document.execCommand("copy");
    }
  } catch (_error) {
    showNotification("Failed to copy link", "error");
    return;
  }

  const originalText = button.textContent;
  button.textContent = "Copied!";
  button.classList.add("copied");

  setTimeout(() => {
    button.textContent = originalText;
    button.classList.remove("copied");
  }, 2000);
}

function initRevealAnimation() {
  const blocks = document.querySelectorAll(".reveal");
  if (blocks.length === 0) return;

  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          entry.target.classList.add("in-view");
          observer.unobserve(entry.target);
        }
      });
    },
    {
      threshold: 0.14,
      rootMargin: "0px 0px -40px 0px",
    },
  );

  blocks.forEach((block, index) => {
    block.style.transitionDelay = `${Math.min(index * 0.08, 0.3)}s`;
    observer.observe(block);
  });
}

// Utility functions
function getFileExtension(filename) {
  const ext = filename.split(".").pop().toUpperCase();
  return ext.substring(0, 3);
}

function isImageFile(filename) {
  return /\.(png|jpe?g|gif|webp|svg|bmp|avif)$/i.test(filename);
}

function formatFileSize(bytes) {
  if (bytes === 0) return "0 Bytes";
  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + " " + sizes[i];
}

function formatDate(timestamp) {
  const date = new Date(timestamp);
  return date.toLocaleString();
}

function showNotification(message, type) {
  const notification = document.createElement("div");
  notification.className =
    type === "success" ? "success-message" : "error-message";
  notification.textContent = message;
  notification.style.position = "fixed";
  notification.style.top = "20px";
  notification.style.right = "20px";
  notification.style.zIndex = "9999";
  notification.style.minWidth = "300px";

  document.body.appendChild(notification);

  setTimeout(() => {
    notification.remove();
  }, 3000);
}
