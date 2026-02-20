(function () {
  const $ = (id) => document.getElementById(id);
  const keys = ["baseUrl", "apiKey", "actor", "role", "scopes"];

  /* ── helpers ──────────────────────────────────────────────── */

  function headers(json) {
    const h = {
      "X-NEXUS-GATEWAY-KEY": $("apiKey").value.trim(),
      "X-NEXUS-ACTOR": $("actor").value.trim(),
      "X-NEXUS-ROLE": $("role").value.trim(),
      "X-NEXUS-SCOPES": $("scopes").value.trim()
    };
    if (json) h["Content-Type"] = "application/json";
    return h;
  }

  async function call(path, opts = {}) {
    const base = $("baseUrl").value.trim().replace(/\/$/, "");
    const res = await fetch(base + path, opts);
    const text = await res.text();
    let body;
    try { body = JSON.parse(text); } catch { body = text; }
    return { status: res.status, body };
  }

  function render(el, data) {
    el.textContent = JSON.stringify(data, null, 2);
  }

  function saveSession() {
    keys.forEach((k) => localStorage.setItem("nexus_admin_" + k, $(k).value));
  }

  function loadSession() {
    keys.forEach((k) => {
      const v = localStorage.getItem("nexus_admin_" + k);
      if (v !== null) $(k).value = v;
    });
    if (!$("hardLimits").value) {
      $("hardLimits").value = JSON.stringify({ tools_max: 20, run_rpm: 300, audit_retention_days: 30 }, null, 2);
    }
  }

  /** Build an HTML table from an array of objects. */
  function buildTable(items, columns, opts) {
    if (!items || items.length === 0) return "<p class='muted-text'>No items found.</p>";
    const actionCol = opts && opts.actions ? "<th>Actions</th>" : "";
    let html = "<table class='data-table'><thead><tr>";
    columns.forEach((c) => { html += "<th>" + esc(c.label) + "</th>"; });
    html += actionCol + "</tr></thead><tbody>";
    items.forEach((item, idx) => {
      html += "<tr>";
      columns.forEach((c) => {
        const val = typeof c.key === "function" ? c.key(item) : item[c.key];
        html += "<td>" + esc(val == null ? "" : String(val)) + "</td>";
      });
      if (opts && opts.actions) {
        html += "<td class='row-actions'>" + opts.actions(item, idx) + "</td>";
      }
      html += "</tr>";
    });
    html += "</tbody></table>";
    return html;
  }

  function esc(s) {
    const d = document.createElement("div");
    d.textContent = s;
    return d.innerHTML;
  }

  function csvToArr(s) {
    return s.split(",").map((x) => x.trim()).filter(Boolean);
  }

  function parseJsonField(el, fallback) {
    try { return JSON.parse(el.value); } catch { return fallback; }
  }

  /* ── tab navigation ──────────────────────────────────────── */

  document.querySelectorAll(".tab").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".tab").forEach((b) => b.classList.remove("active"));
      document.querySelectorAll(".tab-content").forEach((s) => s.classList.remove("active"));
      btn.classList.add("active");
      const target = $("tab-" + btn.dataset.tab);
      if (target) target.classList.add("active");
    });
  });

  /* ── session ─────────────────────────────────────────────── */

  $("btnSave").onclick = saveSession;

  $("btnBootstrap").onclick = async () => {
    saveSession();
    const out = $("bootstrapOut");
    render(out, { loading: true });
    const resp = await call("/v1/admin/bootstrap", { headers: headers(false) });
    render(out, resp);
    if (resp.status === 200) {
      $("planCode").value = resp.body.tenant_settings.plan_code;
      $("hardLimits").value = JSON.stringify(resp.body.tenant_settings.hard_limits || {}, null, 2);
    }
  };

  $("btnSaveSettings").onclick = async () => {
    const out = $("bootstrapOut");
    let hard;
    try { hard = JSON.parse($("hardLimits").value); }
    catch (e) { render(out, { error: "hard_limits must be valid JSON" }); return; }

    const resp = await call("/v1/admin/tenant-settings", {
      method: "PUT",
      headers: headers(true),
      body: JSON.stringify({ plan_code: $("planCode").value, hard_limits: hard })
    });
    render(out, resp);
  };

  $("btnAudit").onclick = async () => render($("auditOut"), await call("/v1/audit?limit=10", { headers: headers(false) }));
  $("btnAdminActivity").onclick = async () => render($("adminActivityOut"), await call("/v1/admin/activity?limit=20", { headers: headers(false) }));

  /* ── REST / MCP demos ────────────────────────────────────── */

  $("btnRunRest").onclick = async () => {
    let payload;
    try { payload = JSON.parse($("restPayload").value); }
    catch { render($("restOut"), { error: "invalid JSON payload" }); return; }
    render($("restOut"), await call("/v1/run", {
      method: "POST",
      headers: headers(true),
      body: JSON.stringify(payload)
    }));
  };

  $("btnRunMcp").onclick = async () => {
    let payload;
    try { payload = JSON.parse($("mcpPayload").value); }
    catch { render($("mcpOut"), { error: "invalid JSON payload" }); return; }
    render($("mcpOut"), await call("/mcp", {
      method: "POST",
      headers: headers(true),
      body: JSON.stringify(payload)
    }));
  };

  /* ═══════════════════════════════════════════════════════════
     1. TOOLS CRUD
     ═══════════════════════════════════════════════════════════ */

  let toolsCache = [];

  $("btnRefreshTools").onclick = loadTools;

  async function loadTools() {
    const resp = await call("/v1/tools", { headers: headers(false) });
    const list = $("toolsList");
    if (resp.status !== 200) { list.innerHTML = "<pre class='mono'>" + esc(JSON.stringify(resp, null, 2)) + "</pre>"; return; }
    toolsCache = resp.body.items || [];
    list.innerHTML = buildTable(toolsCache, [
      { label: "Name", key: "name" },
      { label: "Kind", key: "kind" },
      { label: "Method", key: "method" },
      { label: "Action", key: "action_type" },
      { label: "Risk", key: "risk_level" },
      { label: "Enabled", key: (t) => t.enabled ? "yes" : "no" }
    ], {
      actions: (t) =>
        "<button class='ghost btn-sm' onclick=\"window._nx.viewTool('" + esc(t.name) + "')\">View</button> " +
        "<button class='ghost btn-sm' onclick=\"window._nx.editTool('" + esc(t.name) + "')\">Edit</button>"
    });
  }

  $("btnShowCreateTool").onclick = () => {
    $("toolFormMode").value = "create";
    $("toolFormTitle").textContent = "Create Tool";
    $("tfName").value = "";
    $("tfName").disabled = false;
    $("tfKind").value = "rest";
    $("tfMethod").value = "POST";
    $("tfUrl").value = "";
    $("tfDescription").value = "";
    $("tfActionType").value = "query";
    $("tfClassification").value = "";
    $("tfSensitivity").value = "low";
    $("tfRiskLevel").value = "1";
    $("tfEnabled").value = "true";
    $("tfInputSchema").value = "{}";
    $("toolFormWrap").style.display = "";
  };

  $("btnCancelTool").onclick = () => {
    $("toolFormWrap").style.display = "none";
  };

  $("btnSubmitTool").onclick = async () => {
    const mode = $("toolFormMode").value;
    const inputSchema = parseJsonField($("tfInputSchema"), {});

    if (mode === "create") {
      const payload = {
        name: $("tfName").value.trim(),
        kind: $("tfKind").value,
        method: $("tfMethod").value,
        url: $("tfUrl").value.trim(),
        description: $("tfDescription").value.trim() || null,
        action_type: $("tfActionType").value,
        classification: $("tfClassification").value.trim(),
        sensitivity: $("tfSensitivity").value,
        risk_level: parseInt($("tfRiskLevel").value, 10),
        enabled: $("tfEnabled").value === "true",
        input_schema: inputSchema
      };
      const resp = await call("/v1/tools", {
        method: "POST",
        headers: headers(true),
        body: JSON.stringify(payload)
      });
      if (resp.status === 201 || resp.status === 200) {
        $("toolFormWrap").style.display = "none";
        loadTools();
      } else {
        alert("Error creating tool: " + JSON.stringify(resp.body));
      }
    } else {
      // update
      const name = $("tfName").value.trim();
      const payload = {
        method: $("tfMethod").value,
        url: $("tfUrl").value.trim(),
        description: $("tfDescription").value.trim() || null,
        action_type: $("tfActionType").value,
        classification: $("tfClassification").value.trim(),
        sensitivity: $("tfSensitivity").value,
        risk_level: parseInt($("tfRiskLevel").value, 10),
        enabled: $("tfEnabled").value === "true",
        input_schema: inputSchema
      };
      const resp = await call("/v1/tools/" + encodeURIComponent(name), {
        method: "PUT",
        headers: headers(true),
        body: JSON.stringify(payload)
      });
      if (resp.status === 200) {
        $("toolFormWrap").style.display = "none";
        loadTools();
      } else {
        alert("Error updating tool: " + JSON.stringify(resp.body));
      }
    }
  };

  window._nx = window._nx || {};

  window._nx.viewTool = async (name) => {
    const detail = $("toolDetailOut");
    detail.style.display = "";
    render(detail, { loading: true });
    const resp = await call("/v1/tools/" + encodeURIComponent(name), { headers: headers(false) });
    render(detail, resp.body || resp);
  };

  window._nx.editTool = async (name) => {
    const resp = await call("/v1/tools/" + encodeURIComponent(name), { headers: headers(false) });
    if (resp.status !== 200) { alert("Could not load tool"); return; }
    const t = resp.body;
    $("toolFormMode").value = "update";
    $("toolFormTitle").textContent = "Update Tool: " + t.name;
    $("tfName").value = t.name;
    $("tfName").disabled = true;
    $("tfKind").value = t.kind || "rest";
    $("tfMethod").value = t.method || "POST";
    $("tfUrl").value = t.url || "";
    $("tfDescription").value = t.description || "";
    $("tfActionType").value = t.action_type || "query";
    $("tfClassification").value = t.classification || "";
    $("tfSensitivity").value = t.sensitivity || "low";
    $("tfRiskLevel").value = t.risk_level != null ? t.risk_level : 1;
    $("tfEnabled").value = t.enabled ? "true" : "false";
    $("tfInputSchema").value = JSON.stringify(t.input_schema || {}, null, 2);
    $("toolFormWrap").style.display = "";
  };

  /* ═══════════════════════════════════════════════════════════
     2. POLICY MANAGEMENT
     ═══════════════════════════════════════════════════════════ */

  $("btnRefreshPolicies").onclick = loadPolicies;

  async function loadPolicies() {
    const toolName = $("policyToolName").value.trim();
    if (!toolName) { alert("Enter a tool name first"); return; }
    const resp = await call("/v1/tools/" + encodeURIComponent(toolName) + "/policies", { headers: headers(false) });
    const list = $("policiesList");
    if (resp.status !== 200) { list.innerHTML = "<pre class='mono'>" + esc(JSON.stringify(resp, null, 2)) + "</pre>"; return; }
    const items = resp.body.items || [];
    list.innerHTML = buildTable(items, [
      { label: "ID", key: (p) => p.id.slice(0, 8) },
      { label: "Effect", key: "effect" },
      { label: "Priority", key: "priority" },
      { label: "Reason Template", key: "reason_template" },
      { label: "Enabled", key: (p) => p.enabled ? "yes" : "no" }
    ], {
      actions: (p) =>
        "<button class='ghost btn-sm' onclick=\"window._nx.editPolicy('" + esc(p.id) + "'," + JSON.stringify(JSON.stringify(p)) + ")\">Edit</button>"
    });
  }

  $("btnShowCreatePolicy").onclick = () => {
    $("policyFormMode").value = "create";
    $("policyFormId").value = "";
    $("policyFormTitle").textContent = "Create Policy";
    $("pfEffect").value = "allow";
    $("pfPriority").value = "100";
    $("pfEnabled").value = "true";
    $("pfReasonTemplate").value = "";
    $("pfConditions").value = "{}";
    $("pfLimits").value = "{}";
    $("policyFormWrap").style.display = "";
  };

  $("btnCancelPolicy").onclick = () => {
    $("policyFormWrap").style.display = "none";
  };

  window._nx.editPolicy = (id, jsonStr) => {
    const p = JSON.parse(jsonStr);
    $("policyFormMode").value = "update";
    $("policyFormId").value = id;
    $("policyFormTitle").textContent = "Update Policy: " + id.slice(0, 8);
    $("pfEffect").value = p.effect || "allow";
    $("pfPriority").value = p.priority != null ? p.priority : 100;
    $("pfEnabled").value = p.enabled ? "true" : "false";
    $("pfReasonTemplate").value = p.reason_template || "";
    $("pfConditions").value = JSON.stringify(p.conditions || {}, null, 2);
    $("pfLimits").value = JSON.stringify(p.limits || {}, null, 2);
    $("policyFormWrap").style.display = "";
  };

  $("btnSubmitPolicy").onclick = async () => {
    const mode = $("policyFormMode").value;
    const conditions = parseJsonField($("pfConditions"), {});
    const limits = parseJsonField($("pfLimits"), {});

    if (mode === "create") {
      const toolName = $("policyToolName").value.trim();
      if (!toolName) { alert("Enter a tool name first"); return; }
      const payload = {
        effect: $("pfEffect").value,
        priority: parseInt($("pfPriority").value, 10),
        conditions: conditions,
        limits: limits,
        reason_template: $("pfReasonTemplate").value.trim(),
        enabled: $("pfEnabled").value === "true"
      };
      const resp = await call("/v1/tools/" + encodeURIComponent(toolName) + "/policies", {
        method: "POST",
        headers: headers(true),
        body: JSON.stringify(payload)
      });
      if (resp.status === 201 || resp.status === 200) {
        $("policyFormWrap").style.display = "none";
        loadPolicies();
      } else {
        alert("Error creating policy: " + JSON.stringify(resp.body));
      }
    } else {
      const id = $("policyFormId").value;
      const payload = {
        effect: $("pfEffect").value,
        priority: parseInt($("pfPriority").value, 10),
        conditions: conditions,
        limits: limits,
        reason_template: $("pfReasonTemplate").value.trim(),
        enabled: $("pfEnabled").value === "true"
      };
      const resp = await call("/v1/policies/" + encodeURIComponent(id), {
        method: "PUT",
        headers: headers(true),
        body: JSON.stringify(payload)
      });
      if (resp.status === 200) {
        $("policyFormWrap").style.display = "none";
        loadPolicies();
      } else {
        alert("Error updating policy: " + JSON.stringify(resp.body));
      }
    }
  };

  /* ═══════════════════════════════════════════════════════════
     3. EGRESS ALLOWLIST
     ═══════════════════════════════════════════════════════════ */

  $("btnRefreshEgress").onclick = loadEgress;

  async function loadEgress() {
    const toolName = $("egressToolName").value.trim();
    if (!toolName) { alert("Enter a tool name first"); return; }
    const resp = await call("/v1/tools/" + encodeURIComponent(toolName) + "/egress-rules", { headers: headers(false) });
    const list = $("egressList");
    if (resp.status !== 200) { list.innerHTML = "<pre class='mono'>" + esc(JSON.stringify(resp, null, 2)) + "</pre>"; return; }
    const items = resp.body.items || [];
    if (items.length === 0) {
      list.innerHTML = "<p class='muted-text'>No egress rules.</p>";
      return;
    }
    // items may be simple strings (hosts) or objects
    const rows = items.map((it) => typeof it === "string" ? { host: it, enabled: true } : it);
    list.innerHTML = buildTable(rows, [
      { label: "Host", key: "host" },
      { label: "Enabled", key: (r) => r.enabled ? "yes" : "no" }
    ], {
      actions: (r) =>
        "<button class='ghost btn-sm btn-danger' onclick=\"window._nx.deleteEgress('" + esc(r.host) + "')\">Remove</button>"
    });
  }

  $("btnAddEgress").onclick = async () => {
    const toolName = $("egressToolName").value.trim();
    if (!toolName) { alert("Enter a tool name first"); return; }
    const host = $("efHost").value.trim();
    if (!host) { alert("Enter a host"); return; }
    const payload = {
      host: host,
      enabled: $("efEnabled").value === "true"
    };
    const resp = await call("/v1/tools/" + encodeURIComponent(toolName) + "/egress-rules", {
      method: "POST",
      headers: headers(true),
      body: JSON.stringify(payload)
    });
    if (resp.status === 204 || resp.status === 200) {
      $("efHost").value = "";
      loadEgress();
    } else {
      alert("Error adding egress rule: " + JSON.stringify(resp.body));
    }
  };

  window._nx.deleteEgress = async (host) => {
    if (!confirm("Remove egress rule for: " + host + "?")) return;
    const toolName = $("egressToolName").value.trim();
    const resp = await call("/v1/tools/" + encodeURIComponent(toolName) + "/egress-rules?host=" + encodeURIComponent(host), {
      method: "DELETE",
      headers: headers(false)
    });
    if (resp.status === 204 || resp.status === 200) {
      loadEgress();
    } else {
      alert("Error deleting egress rule: " + JSON.stringify(resp.body));
    }
  };

  /* ═══════════════════════════════════════════════════════════
     4. SECRETS VAULT
     ═══════════════════════════════════════════════════════════ */

  $("btnRefreshSecrets").onclick = loadSecrets;

  async function loadSecrets() {
    const toolName = $("secretsToolName").value.trim();
    if (!toolName) { alert("Enter a tool name first"); return; }
    const resp = await call("/v1/tools/" + encodeURIComponent(toolName) + "/secrets", { headers: headers(false) });
    const list = $("secretsList");
    if (resp.status !== 200) { list.innerHTML = "<pre class='mono'>" + esc(JSON.stringify(resp, null, 2)) + "</pre>"; return; }
    const items = resp.body.items || [];
    list.innerHTML = buildTable(items, [
      { label: "Key Name", key: "key_name" },
      { label: "Type", key: "secret_type" },
      { label: "Enabled", key: (s) => s.enabled ? "yes" : "no" },
      { label: "Updated", key: "updated_at" }
    ], {
      actions: (s) =>
        "<button class='ghost btn-sm btn-danger' onclick=\"window._nx.deleteSecret('" + esc(s.key_name) + "')\">Delete</button>"
    });
  }

  $("btnAddSecret").onclick = async () => {
    const toolName = $("secretsToolName").value.trim();
    if (!toolName) { alert("Enter a tool name first"); return; }
    const payload = {
      secret_type: $("sfType").value,
      key_name: $("sfKeyName").value.trim(),
      value: $("sfValue").value
    };
    if (!payload.key_name || !payload.value) { alert("key_name and value are required"); return; }
    const resp = await call("/v1/tools/" + encodeURIComponent(toolName) + "/secrets", {
      method: "POST",
      headers: headers(true),
      body: JSON.stringify(payload)
    });
    if (resp.status === 200 || resp.status === 201) {
      $("sfKeyName").value = "";
      $("sfValue").value = "";
      loadSecrets();
    } else {
      alert("Error upserting secret: " + JSON.stringify(resp.body));
    }
  };

  window._nx.deleteSecret = async (keyName) => {
    if (!confirm("Delete secret: " + keyName + "?")) return;
    const toolName = $("secretsToolName").value.trim();
    const resp = await call("/v1/tools/" + encodeURIComponent(toolName) + "/secrets?key_name=" + encodeURIComponent(keyName), {
      method: "DELETE",
      headers: headers(false)
    });
    if (resp.status === 204 || resp.status === 200) {
      loadSecrets();
    } else {
      alert("Error deleting secret: " + JSON.stringify(resp.body));
    }
  };

  /* ═══════════════════════════════════════════════════════════
     5. INCIDENTS DASHBOARD
     ═══════════════════════════════════════════════════════════ */

  $("btnRefreshIncidents").onclick = loadIncidents;

  async function loadIncidents() {
    const status = $("incidentStatusFilter").value;
    const qs = status ? "?status=" + status + "&limit=100" : "?limit=100";
    const resp = await call("/v1/incidents" + qs, { headers: headers(false) });
    const list = $("incidentsList");
    if (resp.status !== 200) { list.innerHTML = "<pre class='mono'>" + esc(JSON.stringify(resp, null, 2)) + "</pre>"; return; }
    const items = resp.body.items || [];
    list.innerHTML = buildTable(items, [
      { label: "ID", key: (i) => i.id.slice(0, 8) },
      { label: "Severity", key: "severity" },
      { label: "Status", key: "status" },
      { label: "Title", key: "title" },
      { label: "Opened", key: "opened_at" }
    ], {
      actions: (i) => {
        let btns = "<button class='ghost btn-sm' onclick=\"window._nx.viewIncident('" + esc(i.id) + "')\">View</button>";
        if (i.status === "open") {
          btns += " <button class='ghost btn-sm btn-danger' onclick=\"window._nx.closeIncident('" + esc(i.id) + "')\">Close</button>";
        }
        return btns;
      }
    });
  }

  $("btnShowCreateIncident").onclick = () => {
    $("incidentFormWrap").style.display = "";
  };

  $("btnCancelIncident").onclick = () => {
    $("incidentFormWrap").style.display = "none";
  };

  $("btnSubmitIncident").onclick = async () => {
    const payload = {
      severity: $("ifSeverity").value,
      title: $("ifTitle").value.trim(),
      summary: $("ifSummary").value.trim(),
      related_action_ids: csvToArr($("ifActionIds").value),
      evidence_refs: csvToArr($("ifEvidenceRefs").value)
    };
    if (!payload.title || !payload.summary) { alert("Title and summary are required"); return; }
    const resp = await call("/v1/incidents", {
      method: "POST",
      headers: headers(true),
      body: JSON.stringify(payload)
    });
    if (resp.status === 201 || resp.status === 200) {
      $("incidentFormWrap").style.display = "none";
      $("ifTitle").value = "";
      $("ifSummary").value = "";
      $("ifActionIds").value = "";
      $("ifEvidenceRefs").value = "";
      loadIncidents();
    } else {
      alert("Error creating incident: " + JSON.stringify(resp.body));
    }
  };

  window._nx.viewIncident = async (id) => {
    const detail = $("incidentDetailOut");
    detail.style.display = "";
    render(detail, { loading: true });
    const resp = await call("/v1/incidents/" + encodeURIComponent(id), { headers: headers(false) });
    render(detail, resp.body || resp);
  };

  window._nx.closeIncident = async (id) => {
    if (!confirm("Close incident " + id.slice(0, 8) + "?")) return;
    const resp = await call("/v1/incidents/" + encodeURIComponent(id) + "/close", {
      method: "POST",
      headers: headers(true),
      body: JSON.stringify({})
    });
    if (resp.status === 200) {
      loadIncidents();
    } else {
      alert("Error closing incident: " + JSON.stringify(resp.body));
    }
  };

  /* ═══════════════════════════════════════════════════════════
     6. ACTIONS DASHBOARD
     ═══════════════════════════════════════════════════════════ */

  $("btnRefreshActions").onclick = loadActions;

  async function loadActions() {
    const status = $("actionStatusFilter").value;
    const qs = status ? "?status=" + status + "&limit=100" : "?limit=100";
    const resp = await call("/v1/actions" + qs, { headers: headers(false) });
    const list = $("actionsList");
    if (resp.status !== 200) { list.innerHTML = "<pre class='mono'>" + esc(JSON.stringify(resp, null, 2)) + "</pre>"; return; }
    const items = resp.body.items || [];
    list.innerHTML = buildTable(items, [
      { label: "ID", key: (a) => a.id.slice(0, 8) },
      { label: "Type", key: "action_type" },
      { label: "Scope", key: (a) => a.scope_type + (a.scope_id ? ":" + a.scope_id : "") },
      { label: "Status", key: "status" },
      { label: "TTL", key: (a) => a.ttl_seconds ? a.ttl_seconds + "s" : "-" },
      { label: "Created", key: "created_at" }
    ], {
      actions: (a) => {
        let btns = "<button class='ghost btn-sm' onclick=\"window._nx.viewAction(" + JSON.stringify(JSON.stringify(a)) + ")\">View</button>";
        if (a.status === "active") {
          btns += " <button class='ghost btn-sm btn-danger' onclick=\"window._nx.rollbackAction('" + esc(a.id) + "')\">Rollback</button>";
        }
        return btns;
      }
    });
  }

  window._nx.viewAction = (jsonStr) => {
    const detail = $("actionDetailOut");
    detail.style.display = "";
    render(detail, JSON.parse(jsonStr));
  };

  window._nx.rollbackAction = async (id) => {
    if (!confirm("Rollback action " + id.slice(0, 8) + "?")) return;
    const resp = await call("/v1/actions/rollback", {
      method: "POST",
      headers: headers(true),
      body: JSON.stringify({ action_id: id })
    });
    if (resp.status === 200) {
      loadActions();
    } else {
      alert("Error rolling back action: " + JSON.stringify(resp.body));
    }
  };

  /* ── init ─────────────────────────────────────────────────── */

  loadSession();
})();
