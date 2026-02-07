const DEFAULT_REFRESH_MS = 5000;

const pageTitle = document.getElementById("page-title");
const pageSubtitle = document.getElementById("page-subtitle");
const generatedAt = document.getElementById("generated-at");
const globalStatus = document.getElementById("global-status");
const foldersCount = document.getElementById("folders-count");
const foldersSummary = document.getElementById("folders-summary");
const foldersList = document.getElementById("folders-list");
const deviceList = document.getElementById("device-list");
const remotesCount = document.getElementById("remotes-count");
const remotesSummary = document.getElementById("remotes-summary");
const remotesList = document.getElementById("remotes-list");
let refreshMs = DEFAULT_REFRESH_MS;
let refreshTimer = null;

const BYTES_PER_GIB = 1024 ** 3;

function formatBytes(value) {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let number = Math.max(0, Number(value || 0));
  let unitIndex = 0;
  while (number >= 1024 && unitIndex < units.length - 1) {
    number /= 1024;
    unitIndex += 1;
  }
  return `${number.toFixed(number >= 100 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function formatRate(value) {
  return `${formatBytes(value)}/s`;
}

function formatGiB(value) {
  const gib = Math.max(0, Number(value || 0)) / BYTES_PER_GIB;
  return `${gib.toFixed(1)} GiB`;
}

function formatDate(value) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function formatUptime(seconds) {
  const total = Math.max(0, Number(seconds || 0));
  const days = Math.floor(total / 86400);
  const hours = Math.floor((total % 86400) / 3600);
  const mins = Math.floor((total % 3600) / 60);

  if (days > 0) {
    return `${days}d ${hours}h`;
  }
  if (hours > 0) {
    return `${hours}h ${mins}m`;
  }
  return `${mins}m`;
}

function shortIdentification(id) {
  const raw = String(id || "").trim();
  if (!raw) {
    return "-";
  }
  const firstToken = raw.split("-")[0];
  if (firstToken.length >= 7) {
    return firstToken;
  }
  return raw.slice(0, 7);
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function folderIconSVG() {
  return `<svg viewBox="0 0 16 16" aria-hidden="true" focusable="false"><path fill="currentColor" d="M1.5 3A1.5 1.5 0 0 1 3 1.5h2.6c.5 0 1 .2 1.3.6l.9 1H13A1.5 1.5 0 0 1 14.5 4.5v7A1.5 1.5 0 0 1 13 13H3a1.5 1.5 0 0 1-1.5-1.5v-8z"/></svg>`;
}

function deviceIconSVG() {
  return `<svg viewBox="0 0 16 16" aria-hidden="true" focusable="false"><path fill="currentColor" d="M2 3.5A1.5 1.5 0 0 1 3.5 2h9A1.5 1.5 0 0 1 14 3.5v6a1.5 1.5 0 0 1-1.5 1.5H9.6l.9 1.5H12a.5.5 0 0 1 0 1H4a.5.5 0 0 1 0-1h1.5l.9-1.5H3.5A1.5 1.5 0 0 1 2 9.5v-6zm1.5-.5a.5.5 0 0 0-.5.5v6a.5.5 0 0 0 .5.5h9a.5.5 0 0 0 .5-.5v-6a.5.5 0 0 0-.5-.5h-9z"/></svg>`;
}

function statusClassForGlobal(data) {
  if (!data.source_online) {
    return "status-critical";
  }
  if (data.stale) {
    return "status-warn";
  }
  return "status-ok";
}

function folderProgress(folder) {
  const completionPctRaw = Number(folder.completion_pct);
  const hasPct = Number.isFinite(completionPctRaw);
  const percent = hasPct ? Math.max(0, Math.min(100, Math.floor(completionPctRaw))) : null;
  const remaining = Math.max(0, Number(folder.need_bytes || 0));
  return { percent, remaining };
}

function folderStatus(folder) {
  const state = String(folder.state || "").toLowerCase();
  const localChangesItems = Number(folder.local_changes_items || 0);
  const needItems = Number(folder.need_items || 0);
  const needBytes = Number(folder.need_bytes || 0);
  const progress = folderProgress(folder);

  if (state === "paused") {
    return { label: "Paused", cls: "folder-state-paused", phase: "paused", rightText: "Paused" };
  }
  if (state === "error") {
    return { label: "Error", cls: "folder-state-error", phase: "error", rightText: "Error" };
  }

  if (localChangesItems > 0 && needItems === 0 && needBytes === 0) {
    return {
      label: "Local Additions",
      cls: "folder-state-local",
      phase: "local",
      rightText: "Local Additions",
      progressWidth: 0,
    };
  }

  if (needItems > 0 || needBytes > 0 || state.includes("sync") || state.includes("scan") || state.includes("wait")) {
    const percentText = progress.percent === null ? "" : ` (${progress.percent}%)`;
    const sizeText = progress.remaining > 0 ? `, ${formatGiB(progress.remaining)}` : "";
    return {
      label: "Syncing",
      cls: "folder-state-sync",
      phase: "syncing",
      rightText: `Syncing${percentText}${sizeText}`,
      progressWidth: progress.percent === null ? 8 : progress.percent,
    };
  }

  return { label: "Up to Date", cls: "folder-state-up", phase: "idle", rightText: "Up to Date", progressWidth: 0 };
}

function renderDevice(data) {
  const globalClass = statusClassForGlobal(data);
  globalStatus.className = `status-pill ${globalClass}`;
  globalStatus.textContent = !data.source_online
    ? "Source Offline"
    : data.stale
      ? "Stale"
      : "Healthy";

  const device = data.device || {};
  const rows = [
    ["Name", `<span class="meta-entity"><span class="entity-icon device-icon">${deviceIconSVG()}</span>${escapeHTML(device.name || "-")}</span>`],
    ["Download Rate", formatRate(device.download_bps || 0)],
    ["Upload Rate", formatRate(device.upload_bps || 0)],
    ["Local State (Total)", `${Number(device.local_files_total || 0)} files • ${Number(device.local_dirs_total || 0)} dirs • ~${formatBytes(device.local_bytes_total || 0)}`],
    ["Listeners", `${Number(device.listeners_ok || 0)}/${Number(device.listeners_total || 0)}`],
    ["Discovery", `${Number(device.discovery_ok || 0)}/${Number(device.discovery_total || 0)}`],
    ["Uptime", formatUptime(device.uptime_s || 0)],
    ["Identification", `<span class="meta-id">${escapeHTML(shortIdentification(device.id))}</span>`],
    ["Version", escapeHTML(device.version || "-")],
  ];

  if (!data.source_online && data.source_error) {
    rows.push(["Source Error", escapeHTML(data.source_error)]);
  }

  deviceList.innerHTML = rows
    .map(([k, v]) => `<li><span class="meta-key">${k}</span><span class="meta-value">${v}</span></li>`)
    .join("");
}

function renderFolders(data) {
  const folders = Array.isArray(data.folders) ? data.folders : [];
  foldersCount.textContent = String(folders.length);

  if (folders.length === 0) {
    foldersSummary.textContent = "";
    foldersList.innerHTML = `<div class="empty-state">No folders found.</div>`;
    return;
  }

  let syncing = 0;
  let failed = 0;

  foldersList.innerHTML = folders.map((folder) => {
    const status = folderStatus(folder);
    if (status.phase === "syncing") {
      syncing += 1;
    }
    if (status.phase === "error") {
      failed += 1;
    }

    const syncClass = status.phase === "syncing" ? "syncing" : "";
    const progressWidth = Math.max(0, Math.min(100, Number(status.progressWidth || 0)));

    return `
      <article class="folder-row ${syncClass}" data-folder-id="${escapeHTML(folder.id)}" style="--progress-width:${progressWidth}%">
        <div class="folder-main">
          <div class="folder-left">
            <div class="entity-block">
              <span class="entity-icon folder-icon">${folderIconSVG()}</span>
              <div class="entity-text">
                <p class="folder-name">${escapeHTML(folder.label || folder.id || "Unnamed Folder")}</p>
                <p class="folder-path">${escapeHTML(folder.path || "-")}</p>
              </div>
            </div>
          </div>
          <div class="folder-right ${status.cls}">${escapeHTML(status.rightText)}</div>
        </div>
      </article>
    `;
  }).join("");

  const summaryParts = [];
  if (syncing > 0) {
    summaryParts.push(`${syncing} syncing`);
  }
  if (failed > 0) {
    summaryParts.push(`${failed} with errors`);
  }
  if (summaryParts.length === 0) {
    summaryParts.push("all up to date");
  }
  foldersSummary.textContent = summaryParts.join(" • ");
}

function renderRemotes(data) {
  const remotes = Array.isArray(data.remotes) ? data.remotes : [];
  remotesCount.textContent = String(remotes.length);

  if (remotes.length === 0) {
    remotesSummary.textContent = "";
    remotesList.innerHTML = `<div class="empty-state">No remote devices found.</div>`;
    return;
  }

  let disconnected = 0;
  remotesList.innerHTML = remotes.map((remote) => {
    const isConnected = Boolean(remote.connected);
    if (!isConnected) {
      disconnected += 1;
    }

    const stateClass = isConnected ? "remote-state-up" : "remote-state-down";
    const stateText = isConnected ? "Up to Date" : "Disconnected";
    const details = remote.address || `Last seen ${formatDate(remote.last_seen_at)}`;

    return `
      <article class="remote-row">
        <div class="remote-main">
          <div class="remote-left">
            <div class="entity-block">
              <span class="entity-icon device-icon">${deviceIconSVG()}</span>
              <div class="entity-text">
                <p class="remote-name">${escapeHTML(remote.name || remote.id || "Unknown Device")}</p>
                <p class="remote-sub">${escapeHTML(details || "-")}</p>
              </div>
            </div>
          </div>
          <div class="remote-right ${stateClass}">${escapeHTML(stateText)}</div>
        </div>
      </article>
    `;
  }).join("");

  remotesSummary.textContent = disconnected > 0
    ? `${disconnected} disconnected`
    : "all connected";
}

function render(data) {
  const effectiveTitle = data.page_title || "Syncthing";
  pageTitle.textContent = effectiveTitle;
  document.title = effectiveTitle;
  pageSubtitle.textContent = data.page_subtitle || "Read-Only Dashboard";
  const serverPoll = Number(data.poll_interval_ms || 0);
  if (Number.isFinite(serverPoll) && serverPoll > 0 && serverPoll !== refreshMs) {
    refreshMs = serverPoll;
  }
  generatedAt.textContent = formatDate(data.generated_at);
  renderDevice(data);
  renderFolders(data);
  renderRemotes(data);
}

function scheduleRefresh() {
  if (refreshTimer) {
    clearTimeout(refreshTimer);
  }
  refreshTimer = setTimeout(refresh, refreshMs);
}

async function refresh() {
  try {
    const response = await fetch("/api/v1/dashboard", { cache: "no-store" });
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    const data = await response.json();
    render(data);
  } catch (error) {
    generatedAt.textContent = `Failed to load snapshot: ${error.message}`;
    globalStatus.className = "status-pill status-critical";
    globalStatus.textContent = "Unavailable";
  } finally {
    scheduleRefresh();
  }
}

refresh();
