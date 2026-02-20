(function () {
  const $ = (id) => document.getElementById(id);
  const keys = ["baseUrl", "apiKey", "actor", "role", "scopes"];

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

  $("btnTools").onclick = async () => render($("toolsOut"), await call("/v1/tools", { headers: headers(false) }));
  $("btnAudit").onclick = async () => render($("auditOut"), await call("/v1/audit?limit=10", { headers: headers(false) }));
  $("btnAdminActivity").onclick = async () => render($("adminActivityOut"), await call("/v1/admin/activity?limit=20", { headers: headers(false) }));

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

  loadSession();
})();
