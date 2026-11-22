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
const nodeAction = document.querySelector("#node-action");
const commandForm = document.querySelector("#command-form");
const commandInput = document.querySelector("#command-input");
const commandStatus = document.querySelector("#command-status");
const eventDialog = document.querySelector("#event-dialog");
const eventDialogHeading = document.querySelector("#event-dialog-heading");
const eventDialogSummary = document.querySelector("#event-dialog-summary");
const eventDetails = document.querySelector("#event-details");

let snapshot = null;
let selectedNode = null;
let nodePositions = new Map();
const nodeButtons = new Map();
let refreshTimer = null;
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

function scheduleRefresh() {
  if (refreshTimer !== null) window.clearTimeout(refreshTimer);
  refreshTimer = window.setTimeout(() => {
    refreshTimer = null;
    refresh();
  }, 120);
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
  const nodeIDs = new Set();

  data.nodes.forEach((node, index) => {
    const position = positions[index];
    nodePositions.set(node.id, position);
    nodeIDs.add(node.id);
    const button = nodeButtons.get(node.id) || createNodeButton(node.id);
    button.className = `node ${node.state}${!node.networkReady ? " isolated" : ""}${selectedNode === node.id ? " selected" : ""}`;
    button.style.left = `${position.x}px`;
    button.style.top = `${position.y}px`;
    button.setAttribute("aria-pressed", String(selectedNode === node.id));
    button.setAttribute("aria-label", `${node.id}, ${node.networkReady ? node.state : "isolated"}, term ${node.currentTerm}. Click to view details.`);

    button.querySelector(".node-id").textContent = node.id;
    button.querySelector(".node-role").textContent = node.networkReady ? node.state : "isolated";
    button.querySelector(".node-term").textContent = `term ${node.currentTerm}`;
  });

  nodeButtons.forEach((button, nodeID) => {
    if (!nodeIDs.has(nodeID)) {
      button.remove();
      nodeButtons.delete(nodeID);
    }
  });
}

function createNodeButton(nodeID) {
  const button = document.createElement("button");
  button.type = "button";
  button.append(text("span", "node-id", nodeID), text("span", "node-role", ""), text("span", "node-term", ""));
  button.addEventListener("click", () => selectNode(nodeID));
  nodeLayer.append(button);
  nodeButtons.set(nodeID, button);
  return button;
}

function calculatePositions(count) {
  const width = stage.clientWidth;
  const height = stage.clientHeight;
  const radiusX = Math.min(width * 0.27, 250);
  const radiusY = Math.min(height * 0.34, 142);
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
        const cell = text("span", `log-entry${index + 1 <= node.commitIndex ? " committed" : ""}`, `${entry.command} · term ${entry.term}`);
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
    nodeAction.hidden = true;
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
  nodeAction.hidden = false;
  nodeAction.textContent = node.networkReady ? "Isolate node" : "Reconnect node";
}

function setDetailValues(values) {
  details.querySelectorAll("dd").forEach((element, index) => {
    element.textContent = values[index];
  });
}

function selectNode(nodeID) {
  selectedNode = nodeID;
  render(snapshot);
}

async function setNodeAvailability(node) {
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

nodeAction.addEventListener("click", () => {
  const node = snapshot?.nodes.find((item) => item.id === selectedNode);
  if (node) setNodeAvailability(node);
});

commandForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const button = commandForm.querySelector("button");
  button.disabled = true;
  try {
    const result = await request("/api/commands", {
      method: "POST",
      body: JSON.stringify({command: commandInput.value}),
    });
    commandInput.value = "";
    setCommandStatus(`Accepted by ${result.leader} · index ${result.index} · pending commit`);
    refresh();
  } catch (error) {
    setCommandStatus(error.message, true);
  } finally {
    button.disabled = false;
  }
});

function setCommandStatus(message, isError = false) {
  commandStatus.textContent = message;
  commandStatus.classList.toggle("error", isError);
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
    if (!event.heartbeat || ["dropped", "unreachable", "cancelled", "rejected"].includes(event.stage)) {
      appendEvent(event);
    }
    if (!event.replay && event.stage === "sent") {
      animateMessage(event);
    }
    if (!event.replay && ["replied", "rejected"].includes(event.stage)) scheduleRefresh();
  };
}

function appendEvent(event) {
  eventList.querySelector(".empty-event")?.remove();
  const item = document.createElement("li");
  if (["dropped", "unreachable", "cancelled", "rejected"].includes(event.stage)) {
    item.className = "event-error";
  }
  const followNewest = eventList.scrollHeight - eventList.scrollTop - eventList.clientHeight < 16;
  const button = document.createElement("button");
  button.type = "button";
  button.className = "event-button";
  button.setAttribute("aria-label", `Show details for ${event.from} to ${event.to}: ${event.method}, ${event.stage}`);
  button.addEventListener("click", () => showEventDetails(event));
  const timestamp = document.createElement("time");
  timestamp.dateTime = event.at;
  timestamp.textContent = formatEventTime(event.at);
  const summary = document.createElement("span");
  summary.textContent = `${event.from} → ${event.to} · ${event.method} · ${event.stage}${event.error ? ` (${event.error})` : ""}`;
  button.append(timestamp, summary);
  item.append(button);
  eventList.append(item);
  while (eventList.children.length > 80) {
    eventList.firstElementChild.remove();
  }
  if (followNewest) eventList.scrollTop = eventList.scrollHeight;
}

function showEventDetails(event) {
  eventDialogHeading.textContent = `${event.method} · ${event.stage}`;
  eventDialogSummary.textContent = `${event.from} → ${event.to}`;
  eventDetails.replaceChildren(
    eventDetail("Time", formatEventTime(event.at)),
    eventDetail("Term", event.term),
    eventDetail("Heartbeat", event.heartbeat ? "yes" : "no"),
    eventDetail("RPC request arguments", JSON.stringify(event.args, null, 2), true),
  );
  if (event.error) eventDetails.append(eventDetail("Transport error", event.error));
  if (!eventDialog.open) eventDialog.showModal();
}

function eventDetail(label, value, code = false) {
  const group = document.createElement("div");
  if (code) group.className = "event-args";
  const name = text("dt", "", label);
  const content = document.createElement("dd");
  if (code) {
    const pre = document.createElement("pre");
    pre.textContent = value;
    content.append(pre);
  } else {
    content.textContent = value;
  }
  group.append(name, content);
  return group;
}

function formatEventTime(value) {
  return new Date(value).toLocaleTimeString("en-US", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
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
  ], {duration: reduceMotion.matches ? 1 : 180, easing: "cubic-bezier(.22,.75,.3,1)"});
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
