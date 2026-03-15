/** @format */

document.documentElement.classList.add("js");

const STORAGE_KEYS = {
  botToken: "telerealm.botToken",
  chatId: "telerealm.chatId",
  uploadedFiles: "telerealm.uploadedFiles",
};

let selectedFiles = [];
let botToken = "";
let chatId = "";

// Load saved configuration
window.onload = function () {
  hydrateSavedConfig();

  loadUploadedFiles();
  initRevealAnimation();
  initSetupGuide();
  initLocalNotice();
  initSetupVideo();
  initUploadUI();
  initLocalActions();
};

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

  dropZone.addEventListener("click", () => fileInput.click());

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
  const botTokenInput = document.getElementById("botToken");
  const chatIdInput = document.getElementById("chatId");

  if (!botTokenInput || !chatIdInput) return;

  botToken = botTokenInput.value.trim();
  chatId = chatIdInput.value.trim();

  if (!botToken || !chatId) {
    showNotification("Please enter both Bot Token and Chat ID", "error");
    return;
  }

  localStorage.setItem(STORAGE_KEYS.botToken, botToken);
  localStorage.setItem(STORAGE_KEYS.chatId, chatId);

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
      const result = await uploadFile(file, progressItem);
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
    } catch (error) {
      progressItem.innerHTML += `<div class="error-message">Failed: ${error.message}</div>`;
    }
  }

  // Reset
  selectedFiles = [];
  renderSelectedFiles();
  uploadBtn.disabled = false;
  uploadBtn.textContent = "Upload Files";

  // Reload uploaded files list
  loadUploadedFiles();

  showNotification("Upload completed!", "success");
}

// Upload single file
async function uploadFile(file, progressItem) {
  const formData = new FormData();
  formData.append("chat_id", chatId);
  formData.append("document", file);

  const response = await fetch("/send", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${botToken}`,
    },
    body: formData,
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.message || "Upload failed");
  }
  return await response.json();
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
