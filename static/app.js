/** @format */

let selectedFiles = [];
let botToken = "";
let chatId = "";

// Load saved configuration
window.onload = function () {
  botToken = localStorage.getItem("botToken") || "";
  chatId = localStorage.getItem("chatId") || "";

  if (botToken) document.getElementById("botToken").value = botToken;
  if (chatId) document.getElementById("chatId").value = chatId;

  loadUploadedFiles();
};

// Save configuration
function saveConfig() {
  botToken = document.getElementById("botToken").value.trim();
  chatId = document.getElementById("chatId").value.trim();

  if (!botToken || !chatId) {
    showNotification("Please enter both Bot Token and Chat ID", "error");
    return;
  }

  localStorage.setItem("botToken", botToken);
  localStorage.setItem("chatId", chatId);

  showNotification("Configuration saved successfully!", "success");
}

// Drag and drop handlers
const dropZone = document.getElementById("dropZone");
const fileInput = document.getElementById("fileInput");

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
  fileInput.value = ""; // Reset input
});

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

      // Save to localStorage
      saveUploadedFile({
        name: file.name,
        size: file.size,
        secure_url: result.secure_url,
        fileId: result.file_id,
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
  let uploadedFiles = JSON.parse(localStorage.getItem("uploadedFiles") || "[]");
  uploadedFiles.unshift(fileData);

  // Keep only last 50 files
  if (uploadedFiles.length > 50) {
    uploadedFiles = uploadedFiles.slice(0, 50);
  }

  localStorage.setItem("uploadedFiles", JSON.stringify(uploadedFiles));
}

// Load uploaded files
function loadUploadedFiles() {
  const uploadedFiles = JSON.parse(
    localStorage.getItem("uploadedFiles") || "[]",
  );
  const container = document.getElementById("filesList");

  if (uploadedFiles.length === 0) {
    container.innerHTML =
      '<div class="empty-state">No files uploaded yet</div>';
    return;
  }

  container.innerHTML = uploadedFiles
    .map(
      (file, index) => `
        <div class="uploaded-file-item">
            <div class="uploaded-file-header">
                <span class="file-name-uploaded">${file.name}</span>
                <span class="upload-time">${formatDate(file.timestamp)}</span>
            </div>
            <div class="file-size">${formatFileSize(file.size)}</div>
            <div class="file-link">
                <input type="text" class="link-input" value="${file.secure_url}" readonly id="link-${index}">
                <button class="btn-copy" onclick="copyLink(${index})">Copy</button>
                <button class="btn-download" onclick="window.open('${file.secure_url}', '_blank')">Download</button>
            </div>
        </div>
    `,
    )
    .join("");
}

// Copy link to clipboard
function copyLink(index) {
  const input = document.getElementById(`link-${index}`);
  input.select();
  document.execCommand("copy");

  const btn = event.target;
  const originalText = btn.textContent;
  btn.textContent = "Copied!";
  btn.classList.add("copied");

  setTimeout(() => {
    btn.textContent = originalText;
    btn.classList.remove("copied");
  }, 2000);
}

// Utility functions
function getFileExtension(filename) {
  const ext = filename.split(".").pop().toUpperCase();
  return ext.substring(0, 3);
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
