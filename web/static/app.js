"use strict";

const stage = document.querySelector("#cluster-stage");
const linkLayer = document.querySelector("#link-layer");
const nodeLayer = document.querySelector("#node-layer");
const messageLayer = document.querySelector("#message-layer");
const logTable = document.querySelector("#log-table");
const modeSelect = document.querySelector("#network-mode");
const detailsHeading = document.querySelector("#details-heading");
const details = document.querySelector("#node-details");
const eventList = document.querySelector("#event-list");
const connection = document.querySelector(".connection");
const connectionStatus = document.querySelector("#connection-status");

let snapshot = null;
let selectedNode = null;
let nodePositions = new Map();
const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");

async function request(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {"Content-Type": "application/json", ...(options.headers || {})},
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({error: response.statusText}));
    throw new Error(body.error || "Request failed");
  }
  return response.json();
}

async function refresh() {
  try {
    snapshot = await request("/api/state");
    render(snapshot);
    setConnection(true, "Connected");
  } catch (error) {
    setConnection(false, error.message);
  }
}

function render(data) {
  modeSelect.value = data.network.mode;
  renderNodes(data);
  renderLinks(data);
  renderLogs(data.nodes);
  renderDetails(data.nodes.find((node) => node.id === selectedNode));
}

function renderNodes(data) {
  const positions = calculatePositions(data.nodes.length);
  nodePositions = new Map();
  nodeLayer.replaceChildren();

  data.nodes.forEach((node, index) => {
    const position = positions[index];
    nodePositions.set(node.id, position);
    const button = document.createElement("button");
    button.type = "button";
    button.className = `node ${node.state}${selectedNode === node.id ? " selected" : ""}`;
    button.style.left = `${position.x}px`;
    button.style.top = `${position.y}px`;
    button.setAttribute("aria-pressed", String(!node.networkReady));
    button.setAttribute("aria-label", `${node.id}, ${node.state}, term ${node.currentTerm}. ${node.networkReady ? "Click to isolate" : "Click to reconnect"}`);

    const name = text("span", "node-id", node.id);
    const role = text("span", "node-role", node.networkReady ? node.state : "isolated");
    const term = text("span", "node-term", `term ${node.currentTerm}`);
    button.append(name, role, term);
    button.addEventListener("click", () => toggleNode(node));
    nodeLayer.append(button);
  });
}

function calculatePositions(count) {
  const width = stage.clientWidth;
  const height = stage.clientHeight;
  const radiusX = Math.min(width * 0.34, 300);
  const radiusY = Math.min(height * 0.35, 150);
  return Array.from({length: count}, (_, index) => {
    const angle = -Math.PI / 2 + (index * 2 * Math.PI) / count;
    return {
      x: width / 2 + Math.cos(angle) * radiusX,
      y: height / 2 + Math.sin(angle) * radiusY,
    };
  });
}

function renderLinks(data) {
  const namespace = "http://www.w3.org/2000/svg";
  linkLayer.replaceChildren();
  for (let i = 0; i < data.nodes.length; i += 1) {
    for (let j = i + 1; j < data.nodes.length; j += 1) {
      const from = data.nodes[i];
      const to = data.nodes[j];
      const start = nodePositions.get(from.id);
      const end = nodePositions.get(to.id);
      const line = document.createElementNS(namespace, "line");
      line.setAttribute("x1", start.x);
      line.setAttribute("y1", start.y);
      line.setAttribute("x2", end.x);
      line.setAttribute("y2", end.y);
      if (!from.networkReady || !to.networkReady || isLinkDisabled(data.network.disabledLinks, from.id, to.id)) {
        line.classList.add("disabled");
      }
      linkLayer.append(line);
    }
  }
}

function isLinkDisabled(disabledLinks, left, right) {
  return disabledLinks.some((link) =>
    (link.from === left && link.to === right) || (link.from === right && link.to === left));
}

function renderLogs(nodes) {
  logTable.replaceChildren();
  nodes.forEach((node) => {
    const row = document.createElement("div");
    row.className = "log-row";
    row.append(text("span", "log-label", node.id));
    if (node.log.length === 0) {
      row.append(text("span", "log-empty", "empty"));
    } else {
      node.log.forEach((entry, index) => {
        const cell = text("span", `log-entry${index + 1 <= node.commitIndex ? " committed" : ""}`, `${index + 1} · t${entry.term}`);
        cell.title = entry.command;
        row.append(cell);
      });
    }
    logTable.append(row);
  });
}

function renderDetails(node) {
  if (!node) {
    detailsHeading.textContent = "Select a node";
    setDetailValues(["—", "—", "—", "—", "—"]);
    return;
  }
  detailsHeading.textContent = node.id;
  setDetailValues([
    node.networkReady ? node.state : "isolated",
    node.currentTerm,
    node.votedFor || "—",
    node.commitIndex,
    node.lastApplied,
  ]);
}

function setDetailValues(values) {
  details.querySelectorAll("dd").forEach((element, index) => {
    element.textContent = values[index];
  });
}

async function toggleNode(node) {
  selectedNode = node.id;
  try {
    snapshot = await request("/api/nodes", {
      method: "POST",
      body: JSON.stringify({id: node.id, enabled: !node.networkReady}),
    });
    render(snapshot);
  } catch (error) {
    setConnection(false, error.message);
  }
}

modeSelect.addEventListener("change", async () => {
  modeSelect.disabled = true;
  try {
    snapshot = await request("/api/network", {
      method: "POST",
      body: JSON.stringify({mode: modeSelect.value}),
    });
    render(snapshot);
  } catch (error) {
    setConnection(false, error.message);
  } finally {
    modeSelect.disabled = false;
  }
});

function connectEvents() {
  const source = new EventSource("/api/events");
  source.onopen = () => {
    setConnection(true, "Connected");
    refresh();
  };
  source.onerror = () => setConnection(false, "Reconnecting…");
  source.onmessage = (message) => {
    const event = JSON.parse(message.data);
    document.querySelector(".stage-note")?.remove();
    appendEvent(event);
    if (event.stage === "sent" || event.stage === "replied") {
      animateMessage(event);
    }
    if (["delivered", "replied", "rejected"].includes(event.stage)) refresh();
  };
}

function appendEvent(event) {
  eventList.querySelector(".empty-event")?.remove();
  const item = document.createElement("li");
  if (["dropped", "unreachable", "cancelled", "rejected"].includes(event.stage)) {
    item.className = "event-error";
  }
  const timestamp = document.createElement("time");
  timestamp.dateTime = event.at;
  timestamp.textContent = new Date(event.at).toLocaleTimeString("en-US", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
  const summary = document.createElement("span");
  summary.textContent = `${event.from} → ${event.to} · ${event.method} · ${event.stage}${event.error ? ` (${event.error})` : ""}`;
  item.append(timestamp, summary);
  eventList.prepend(item);
  while (eventList.children.length > 80) {
    eventList.lastElementChild.remove();
  }
}

function animateMessage(event) {
  const start = nodePositions.get(event.from);
  const end = nodePositions.get(event.to);
  if (!start || !end) return;

  const message = document.createElement("span");
  message.className = `message${event.method === "AppendEntries" ? " append" : ""}`;
  messageLayer.append(message);
  const animation = message.animate([
    {transform: `translate(${start.x}px, ${start.y}px)`, opacity: 0.9},
    {transform: `translate(${end.x}px, ${end.y}px)`, opacity: 0.9},
  ], {duration: reduceMotion.matches ? 1 : 520, easing: "cubic-bezier(.22,.75,.3,1)"});
  animation.finished.finally(() => message.remove());
}

function setConnection(online, label) {
  connection.classList.toggle("online", online);
  connection.classList.toggle("offline", !online);
  connectionStatus.textContent = label;
}

function text(tag, className, content) {
  const element = document.createElement(tag);
  element.className = className;
  element.textContent = content;
  return element;
}

window.addEventListener("resize", () => snapshot && render(snapshot));
refresh();
connectEvents();
