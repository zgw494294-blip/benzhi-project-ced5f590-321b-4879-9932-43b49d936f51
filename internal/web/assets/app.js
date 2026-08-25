(() => {
  "use strict";
  const $ = (selector) => document.querySelector(selector);
  const $$ = (selector) => [...document.querySelectorAll(selector)];
  const state = { case: null, details: null, cases: [] };
  const statusLabels = {
    DRAFT: "草拟", SURVEY_COMPLETE: "调查完成", IN_REVIEW: "送审中",
    REMEDIATION: "整改中", REVIEW_PASSED: "复核通过", FROZEN: "已冻结", RELEASED: "已放行"
  };
  const areaLabels = { CANOPY: "树冠", TRUNK: "主干", ROOT_ZONE: "根区", ENVIRONMENT: "周边环境" };
  const decisionLabels = { PASS: "通过", RETURN: "退回", PENDING: "待复验", FAIL: "不通过" };
  const priorityLabels = { HIGH: "高优先级", MEDIUM: "中优先级", ROUTINE: "常规优先级" };
  const todoLabels = { awaitingEvidence: "待提交证据", awaitingVerify: "待复验", verificationFail: "复验未通过", closed: "已关闭" };
  const windowLabels = { NOT_STARTED: "未到作业窗口", ACTIVE: "作业窗口内", EXPIRED: "作业窗口已过" };

  function notice(message, error = false) {
    const element = $("#notice");
    element.textContent = message;
    element.className = `notice ${error ? "error" : "show"}`;
    window.clearTimeout(notice.timer);
    notice.timer = window.setTimeout(() => { element.className = "notice"; }, 5500);
  }

  function newKey() {
    return window.crypto?.randomUUID?.() || `web-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }

  function meta(role) {
    return {
      actor: $("#actor").value.trim(),
      role: role || $("#role").value,
      expectedVersion: state.case?.version || 0,
      idempotencyKey: newKey()
    };
  }

  async function api(path, options = {}) {
    const response = await fetch(path, {
      ...options,
      headers: options.body ? { "Content-Type": "application/json", ...(options.headers || {}) } : options.headers
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error?.message || `请求失败（${response.status}）`);
    return payload;
  }

  async function refreshCases(selectId) {
    const payload = await api("/api/v1/cases");
    state.cases = payload.items || [];
    const picker = $("#case-picker");
    const current = selectId || state.case?.id || picker.value;
    picker.innerHTML = '<option value="">请选择或新建档案</option>' + state.cases.map(item =>
      `<option value="${escapeHTML(item.id)}">${escapeHTML(item.treeCode)} · ${escapeHTML(statusLabels[item.status] || item.status)}</option>`
    ).join("");
    picker.value = current;
  }

  async function loadCase(id) {
    if (!id) { state.case = null; state.details = null; render(); return; }
    state.details = await api(`/api/v1/cases/${encodeURIComponent(id)}`);
    state.case = state.details.case;
    await refreshCases(id);
    render();
  }

  async function write(path, payload, method = "POST") {
    const result = await api(path, { method, body: JSON.stringify(payload) });
    state.case = result;
    await loadCase(result.id);
    notice("操作已保存，聚合版本已更新。幂等键与审计事件已记录。");
  }

  function escapeHTML(value) {
    return String(value ?? "").replace(/[&<>'"]/g, char => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" })[char]);
  }

  function formData(form) { return Object.fromEntries(new FormData(form).entries()); }
  function iso(value) { return value ? new Date(value).toISOString() : ""; }
  function formatTime(value) { return value ? new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) : "—"; }
  function withError(task) { return task().catch(error => notice(error.message, true)); }

  function render() {
    const item = state.case;
    $("#operation-area").classList.toggle("muted-area", !item);
    if (!item) {
      $("#case-summary").className = "case-summary empty";
      $("#case-summary").textContent = "尚未选择作业档案";
      $$(".operation").forEach(node => node.classList.remove("enabled"));
      renderCollections({ surveys: [], risks: [], measures: [], findings: [] });
      renderSurveyPreflight(null);
      renderRiskWorklist(null);
      renderBatchReviews({ measures: [], findings: [], submittedRevision: 0 }, null);
      renderRemediationTodos(null);
      return;
    }
    $("#case-summary").className = "case-summary";
    $("#case-summary").innerHTML = `<strong>${escapeHTML(item.treeCode)}</strong> · ${escapeHTML(item.location)}<br>状态：${escapeHTML(statusLabels[item.status] || item.status)}　版本：v${item.version}　责任人：${escapeHTML(item.owner)}`;
    const allowed = status => {
      if (status === item.status) return true;
      if (status === "SURVEY_COMPLETE" && item.status === "REMEDIATION") return true;
      return false;
    };
    $$(".operation").forEach(node => {
      const statuses = (node.dataset.available || "").split(",");
      node.classList.toggle("enabled", !node.dataset.available || statuses.some(allowed));
      node.querySelectorAll("button, input, textarea, select").forEach(control => {
        if (!node.dataset.available) return;
        if (control.closest(".serial-lookup")) return;
        control.disabled = !statuses.some(allowed);
      });
    });
    $("#freeze").disabled = item.status !== "REVIEW_PASSED";
    $("#approve").disabled = item.status !== "FROZEN";
    $("#verify-permit").disabled = item.status !== "RELEASED";
    renderStages(item.status);
    renderCollections(item);
    renderSurveyPreflight(state.details?.surveyPreflight);
    renderRiskWorklist(state.details?.riskWorklist);
    renderBatchReviews(item, state.details?.reviewProgress);
    renderRemediationTodos(state.details?.remediationTodos);
    setFormEnabled("#risk-batch-form", item.status === "SURVEY_COMPLETE");
    setFormEnabled("#review-form", item.status === "IN_REVIEW" || item.status === "REMEDIATION");
    setFormEnabled("#batch-review-form", item.status === "IN_REVIEW" || item.status === "REMEDIATION");
    setFormEnabled("#remediation-form", item.status === "REMEDIATION");
    setFormEnabled("#verify-form", item.status === "REMEDIATION");
    renderPermit(item, state.details?.verification);
    renderAudit(state.details?.auditTrail || []);
  }

  function renderStages(status) {
    const order = ["DRAFT", "SURVEY_COMPLETE", "IN_REVIEW", "FROZEN", "RELEASED"];
    let normalized = status;
    if (status === "REMEDIATION" || status === "REVIEW_PASSED") normalized = "IN_REVIEW";
    $$(".workflow div").forEach(node => {
      const stage = node.dataset.stage;
      node.classList.toggle("active", stage === normalized);
      node.classList.toggle("complete", order.indexOf(stage) < order.indexOf(normalized));
    });
  }

  function renderCollections(item) {
    $("#survey-list").innerHTML = (item.surveys || []).map(row =>
      `<div class="record"><strong>${escapeHTML(areaLabels[row.area] || row.area)} · ${escapeHTML(row.id)}</strong><span>${escapeHTML(row.notes)}<br><small>${escapeHTML(row.evidenceRefs.join("；"))}${row.correctionReason ? `<br>更正理由：${escapeHTML(row.correctionReason)}` : ""}</small></span><span class="tag">${row.supersededById ? `已由 ${escapeHTML(row.supersededById)} 取代` : "当前有效"} · ${escapeHTML(row.severity)}</span></div>`
    ).join("") || '<p class="hint">尚无调查记录。</p>';
    $("#measure-list").innerHTML = (item.measures || []).map(row =>
      `<div class="record"><strong>#${row.sequence} · 修订 ${row.revision}</strong><span>${escapeHTML(row.action)}<br><small>验收：${escapeHTML(row.acceptanceCriteria)}</small></span><span class="tag">${escapeHTML(row.riskId)}</span></div>`
    ).join("") || '<p class="hint">尚无处置措施。</p>';
    $("#finding-list").innerHTML = (item.findings || []).map(row =>
      `<div class="record"><strong>${escapeHTML(decisionLabels[row.decision] || row.decision)}</strong><span>${escapeHTML(row.issue || "无退回问题")}<br><small>整改证据：${escapeHTML(row.remediationEvidence || "—")}</small></span><span class="tag">复验 ${escapeHTML(decisionLabels[row.verificationDecision] || row.verificationDecision)}</span></div>`
    ).join("") || '<p class="hint">尚无复核结论。</p>';
    setOptions("#measure-risk", item.risks || [], risk => risk.id, risk => `${risk.category} · ${risk.id}`);
    setOptions("#correction-survey", (item.surveys || []).filter(survey => !survey.supersededById), survey => survey.id, survey => `${areaLabels[survey.area] || survey.area} · ${survey.id}`);
    const revision = item.submittedRevision ? item.submittedRevision + 1 : 1;
    $("#measure-form [name=revision]").value = revision;
    $("#submit-review-form [name=revision]").value = revision;
    const reviewed = new Set((item.findings || []).map(finding => finding.measureId));
    setOptions("#review-measure", (item.measures || []).filter(measure => measure.revision === item.submittedRevision && !reviewed.has(measure.id)), measure => measure.id, measure => `#${measure.sequence} ${measure.action}`);
    const returned = (item.findings || []).filter(finding => finding.decision === "RETURN" && finding.verificationDecision !== "PASS");
    setOptions("#remediation-finding", returned, finding => finding.id, finding => finding.issue);
    setOptions("#verify-finding", returned.filter(finding => finding.remediationEvidence), finding => finding.id, finding => finding.issue);
  }

  function renderSurveyPreflight(preflight) {
    const panel = $("#survey-preflight");
    if (!preflight) { panel.innerHTML = '<p class="hint">选择作业后显示快照就绪预检。</p>'; return; }
    const areas = (preflight.areas || []).map(area => `${areaLabels[area.area] || area.area}：有效 ${area.effective?.length || 0} 条，历史 ${area.history?.length || 0} 条`).join("；");
    const blockers = (preflight.blockers || []).map(value => `<li>${escapeHTML(value)}</li>`).join("");
    panel.innerHTML = `<strong>快照就绪预检：${preflight.ready ? "通过" : "存在阻断"}</strong><p class="hint">${escapeHTML(areas)}</p>${blockers ? `<ul>${blockers}</ul>` : '<p class="hint">四个区域及证据检查均已通过，可以确认现场快照。</p>'}`;
    $("#complete-survey").disabled = state.case?.status !== "DRAFT" || !preflight.ready;
  }

  function renderRiskWorklist(worklist) {
    const items = worklist?.items || [];
    $("#risk-stats").innerHTML = (worklist?.stats || []).map(stat => `<span class="tag">${escapeHTML(priorityLabels[stat.priority] || stat.priority)} ${stat.riskCount} 项 · 措施覆盖 ${stat.coveredCount} 项</span>`).join("");
    $("#risk-list").innerHTML = items.map(item => {
      const row = item.risk;
      return `<label class="record batch-record"><input class="risk-select" type="checkbox" data-risk-id="${escapeHTML(row.id)}"><strong>${escapeHTML(priorityLabels[item.priority] || item.priority)}<br>${escapeHTML(row.category)}</strong><span>${escapeHTML(row.rationale)}<br><small>${escapeHTML(row.id)} · ${item.covered ? "措施已覆盖" : "措施未覆盖"}</small><input class="batch-reason" placeholder="本项调整理由"></span><select class="batch-urgency"><option value="ROUTINE" ${row.urgency === "ROUTINE" ? "selected" : ""}>常规</option><option value="SOON" ${row.urgency === "SOON" ? "selected" : ""}>尽快</option><option value="IMMEDIATE" ${row.urgency === "IMMEDIATE" ? "selected" : ""}>立即</option></select></label>`;
    }).join("") || '<p class="hint">尚未形成风险项。</p>';
  }

  function renderBatchReviews(item, progress) {
    const reviewed = new Set((item.findings || []).map(finding => finding.measureId));
    const measures = (item.measures || []).filter(measure => measure.revision === item.submittedRevision && !reviewed.has(measure.id));
    $("#batch-review-list").innerHTML = measures.map(measure => `<label class="record batch-record"><input class="review-select" type="checkbox" data-measure-id="${escapeHTML(measure.id)}"><strong>#${measure.sequence}</strong><span>${escapeHTML(measure.action)}<input class="review-issue" placeholder="退回时填写具体问题"></span><select class="review-decision"><option value="PASS">通过</option><option value="RETURN">退回</option></select></label>`).join("") || '<p class="hint">当前修订没有待复核措施。</p>';
    const heading = $("#batch-review-form h3");
    if (heading && progress) heading.textContent = `当前修订批量结论（已复核 ${progress.reviewed}/${progress.total}，通过 ${progress.passed}，退回 ${progress.returned}，未复核 ${progress.unreviewed}）`;
  }

  function renderRemediationTodos(todos) {
    const groups = Object.entries(todoLabels).map(([key, label]) => {
      const records = todos?.[key] || [];
      if (!records.length) return `<div class="record"><strong>${label}</strong><span>暂无</span><span class="tag">0</span></div>`;
      return records.map(item => `<div class="record"><strong>${label}<br>#${item.measure.sequence}</strong><span>${escapeHTML(item.finding.issue)}<br><small>措施：${escapeHTML(item.measure.action)}<br>整改证据：${escapeHTML(item.finding.remediationEvidence || "—")}</small></span><span class="tag">${escapeHTML(decisionLabels[item.finding.verificationDecision] || item.finding.verificationDecision)}</span></div>`).join("");
    });
    $("#remediation-todos").innerHTML = groups.join("");
  }

  function setOptions(selector, items, value, label) {
    const element = $(selector);
    if (!element) return;
    element.innerHTML = items.map(item => `<option value="${escapeHTML(value(item))}">${escapeHTML(label(item))}</option>`).join("");
  }

  function setFormEnabled(selector, enabled) {
    const form = $(selector);
    if (form) form.querySelectorAll("button, input, textarea, select").forEach(control => { control.disabled = !enabled; });
  }

  function renderPermit(item, verification) {
    const card = $("#permit-card");
    if (!item.permit) { card.className = "permit-card empty"; card.textContent = item.frozen ? `版本 v${item.frozen.frozenVersion} 已冻结，待批准签发。摘要：${item.frozen.contentDigest}` : "尚无放行凭据"; return; }
    card.className = `permit-card ${verification?.valid ? "valid" : ""}`;
    const windowResult = verification?.windowStatus ? `<br>作业窗口：${escapeHTML(windowLabels[verification.windowStatus] || verification.windowStatus)} · ${verification.readyToStart ? "可开工" : "不可开工"}` : "";
    card.innerHTML = `<strong>开工放行凭据 #${String(item.permit.serialNumber).padStart(6, "0")}</strong><br>古树：${escapeHTML(item.treeCode)}　冻结版本：v${item.permit.frozenVersion}<br>批准人：${escapeHTML(item.permit.approvedBy)}　签发时间：${formatTime(item.permit.issuedAt)}<span class="digest">SHA-256 ${escapeHTML(item.permit.contentDigest)}</span><strong>${escapeHTML(verification?.message || "待核验")}</strong>${windowResult}`;
  }

  function renderSerialPermit(result) {
    const panel = $("#serial-permit-result");
    panel.className = `permit-card ${result.valid && result.windowStatus === "ACTIVE" ? "valid" : ""}`;
    panel.innerHTML = `<strong>凭据 #${String(result.serialNumber).padStart(6, "0")}：${escapeHTML(result.valid ? "内容完整" : "完整性失败")}</strong><br>${escapeHTML(result.message)}<br>作业窗口：${escapeHTML(windowLabels[result.windowStatus] || result.windowStatus)}（${formatTime(result.workWindowStart)} 至 ${formatTime(result.workWindowEnd)}）<br><strong>${result.readyToStart ? "可开工" : "不可开工"}</strong><br>古树：${escapeHTML(result.treeCode)} · ${escapeHTML(result.location)}　责任人：${escapeHTML(result.owner)}<br>批准人：${escapeHTML(result.approvedBy)}　冻结版本：v${result.frozenVersion}<span class="digest">SHA-256 ${escapeHTML(result.expectedDigest || result.calculatedDigest || "—")}</span><button class="button secondary open-verified-case" data-case-id="${escapeHTML(result.caseId)}" type="button">查看对应作业与审计轨迹</button>`;
  }

  function renderAudit(events) {
    $("#audit-list").innerHTML = events.map(event =>
      `<div class="timeline-item"><strong>${escapeHTML(event.type)}</strong> · v${event.version}<br>${escapeHTML(event.actor)}（${escapeHTML(event.role)}） · ${escapeHTML(event.beforeStatus || "新建")} → ${escapeHTML(event.afterStatus)}<time>${formatTime(event.occurredAt)}</time></div>`
    ).join("") || '<p class="hint">选择作业后显示只追加审计轨迹。</p>';
  }

  $("#create-form").addEventListener("submit", event => withError(async () => {
    event.preventDefault();
    const data = formData(event.currentTarget);
    const payload = { ...meta("PATROL"), ...data, workWindowStart: iso(data.workWindowStart), workWindowEnd: iso(data.workWindowEnd) };
    const result = await api("/api/v1/cases", { method: "POST", body: JSON.stringify(payload) });
    await loadCase(result.id);
    event.currentTarget.reset();
    notice("草拟作业档案已创建。请完成四类现场调查。");
  }));

  $("#survey-form").addEventListener("submit", event => withError(async () => {
    event.preventDefault(); const data = formData(event.currentTarget);
    await write(`/api/v1/cases/${state.case.id}/surveys`, { ...meta("PATROL"), ...data, evidenceRefs: data.evidence.split(/\n+/).map(value => value.trim()).filter(Boolean), evidence: undefined });
    event.currentTarget.reset();
  }));
  $("#survey-correction-form").addEventListener("submit", event => withError(async () => {
    event.preventDefault(); const data = formData(event.currentTarget);
    await write(`/api/v1/cases/${state.case.id}/surveys/corrections`, { ...meta("PATROL"), ...data, evidenceRefs: data.evidence.split(/\n+/).map(value => value.trim()).filter(Boolean), evidence: undefined });
    event.currentTarget.reset();
  }));
  $("#correction-survey").addEventListener("change", event => {
    const survey = state.case?.surveys?.find(row => row.id === event.target.value);
    if (survey) $("#survey-correction-form [name=area]").value = survey.area;
  });
  $("#complete-survey").addEventListener("click", () => withError(() => write(`/api/v1/cases/${state.case.id}/survey/complete`, meta("PATROL"))));
  $("#generate-risks").addEventListener("click", () => withError(() => write(`/api/v1/cases/${state.case.id}/risks/generate`, meta("PLANNER"))));
  $("#risk-form").addEventListener("submit", event => withError(async () => { event.preventDefault(); await write(`/api/v1/cases/${state.case.id}/risks`, { ...meta("PLANNER"), ...formData(event.currentTarget) }); event.currentTarget.reset(); }));
  $("#risk-batch-form").addEventListener("submit", event => withError(async () => {
    event.preventDefault();
    const adjustments = [...event.currentTarget.querySelectorAll(".risk-select:checked")].map(input => {
      const row = input.closest(".batch-record");
      return { riskId: input.dataset.riskId, urgency: row.querySelector(".batch-urgency").value, reason: row.querySelector(".batch-reason").value.trim() };
    });
    await write(`/api/v1/cases/${state.case.id}/risks/urgency/batch`, { ...meta("PLANNER"), adjustments });
  }));
  $("#measure-form").addEventListener("submit", event => withError(async () => { event.preventDefault(); const data = formData(event.currentTarget); await write(`/api/v1/cases/${state.case.id}/measures`, { ...meta("PLANNER"), ...data, revision: Number(data.revision), sequence: Number(data.sequence) }); }));
  $("#submit-review-form").addEventListener("submit", event => withError(async () => { event.preventDefault(); const data = formData(event.currentTarget); await write(`/api/v1/cases/${state.case.id}/review/submit`, { ...meta("PLANNER"), revision: Number(data.revision) }); }));
  $("#review-form").addEventListener("submit", event => withError(async () => { event.preventDefault(); await write(`/api/v1/cases/${state.case.id}/reviews`, { ...meta("REVIEWER"), ...formData(event.currentTarget) }); event.currentTarget.reset(); }));
  $("#batch-review-form").addEventListener("submit", event => withError(async () => {
    event.preventDefault();
    const items = [...event.currentTarget.querySelectorAll(".review-select:checked")].map(input => {
      const row = input.closest(".batch-record");
      return { measureId: input.dataset.measureId, decision: row.querySelector(".review-decision").value, issue: row.querySelector(".review-issue").value.trim() };
    });
    await write(`/api/v1/cases/${state.case.id}/reviews/batch`, { ...meta("REVIEWER"), items });
  }));
  $("#remediation-form").addEventListener("submit", event => withError(async () => { event.preventDefault(); await write(`/api/v1/cases/${state.case.id}/remediations`, { ...meta("PLANNER"), ...formData(event.currentTarget) }); }));
  $("#verify-form").addEventListener("submit", event => withError(async () => { event.preventDefault(); await write(`/api/v1/cases/${state.case.id}/remediations/verify`, { ...meta("REVIEWER"), ...formData(event.currentTarget) }); }));
  $("#freeze").addEventListener("click", () => withError(() => write(`/api/v1/cases/${state.case.id}/freeze`, meta("REVIEWER"))));
  $("#approve").addEventListener("click", () => withError(() => write(`/api/v1/cases/${state.case.id}/approve`, meta("REVIEWER"))));
  $("#verify-permit").addEventListener("click", () => withError(async () => { const result = await api(`/api/v1/cases/${state.case.id}/permit/verify`); state.details.verification = result; renderPermit(state.case, result); notice(result.message, !result.valid); }));
  $("#serial-permit-form").addEventListener("submit", event => withError(async () => {
    event.preventDefault(); const serial = formData(event.currentTarget).serial;
    const result = await api(`/api/v1/permits/${encodeURIComponent(serial)}/verify`);
    renderSerialPermit(result);
    notice(result.readyToStart ? "凭据完整且处于作业窗口，可开工。" : "核验完成：当前不可开工。", !result.readyToStart);
  }));
  $("#serial-permit-result").addEventListener("click", event => {
    const button = event.target.closest(".open-verified-case");
    if (button) withError(() => loadCase(button.dataset.caseId));
  });
  $("#case-picker").addEventListener("change", event => withError(() => loadCase(event.target.value)));
  $("#refresh").addEventListener("click", () => withError(async () => { await refreshCases(); if (state.case) await loadCase(state.case.id); notice("数据已刷新。"); }));

  withError(async () => { await refreshCases(); render(); });
})();
