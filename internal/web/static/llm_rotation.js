(function () {
	const EMPTY_PAIR_VALUE = "0:0";

	function init(root) {
		const configRaw = root.dataset.config;
		if (!configRaw) return;
		let config;
		try {
			config = JSON.parse(configRaw);
		} catch {
			return;
		}
		if (!config || typeof config !== "object" || !Array.isArray(config.options) ||
			!config.labels || typeof config.fieldName !== "string" || !config.fieldName ||
			typeof config.labels.clearRow !== "string") {
			return;
		}

		const fieldName = config.fieldName;
		const requireMin = config.requireMin;
		const options = config.options;
		const labels = config.labels;
		const rowsContainer = root.querySelector(".llm-rotation-rows");
		const addBtn = root.querySelector(".llm-rotation-add-btn");
		const noAddHint = root.querySelector(".llm-rotation-no-add");
		const minHint = root.querySelector(".llm-rotation-min");
		const dialog = root.querySelector(".llm-rotation-dialog");
		const modalList = root.querySelector(".llm-rotation-modal-list");
		const selectAllBtn = root.querySelector(".llm-rotation-select-all");
		const clearSelBtn = root.querySelector(".llm-rotation-clear-sel");
		const cancelBtn = root.querySelector(".llm-rotation-cancel");
		const confirmBtn = root.querySelector(".llm-rotation-confirm");
		if (!rowsContainer || !addBtn || !noAddHint || !dialog || !modalList ||
			!selectAllBtn || !clearSelBtn || !cancelBtn || !confirmBtn) {
			return;
		}

		const form = root.closest("form");
		const saveBtn = form ? form.querySelector("[data-llm-gated-save]") : null;

		const optMap = new Map();
		options.forEach(function (o) {
			optMap.set(o.value, o.label);
		});

		function getSelectedValues() {
			const vals = [];
			rowsContainer.querySelectorAll("select.llm-rotation-row").forEach(function (sel) {
				const v = sel.value;
				if (v && v !== EMPTY_PAIR_VALUE) vals.push(v);
			});
			return vals;
		}

		function rebuildSelect(select, currentValue) {
			const selectedElse = getSelectedValues().filter(function (v) {
				return v !== currentValue;
			});
			select.replaceChildren();
			const clearOpt = document.createElement("option");
			clearOpt.value = EMPTY_PAIR_VALUE;
			clearOpt.textContent = labels.clearRow;
			select.appendChild(clearOpt);
			options.forEach(function (o) {
				if (selectedElse.indexOf(o.value) !== -1) return;
				const opt = document.createElement("option");
				opt.value = o.value;
				opt.textContent = o.label;
				select.appendChild(opt);
			});
			if (currentValue && currentValue !== EMPTY_PAIR_VALUE) {
				select.value = currentValue;
				if (select.value !== currentValue) {
					const fallback = document.createElement("option");
					fallback.value = currentValue;
					fallback.textContent = optMap.get(currentValue) || currentValue;
					select.appendChild(fallback);
					select.value = currentValue;
				}
			}
		}

		function onRowChange(e) {
			const select = e.target;
			if (select.value === EMPTY_PAIR_VALUE) {
				select.remove();
			}
			updateAll();
		}

		function wireRow(select, initialValue) {
			rebuildSelect(select, initialValue);
			if (initialValue && initialValue !== EMPTY_PAIR_VALUE) select.value = initialValue;
			select.addEventListener("change", onRowChange);
		}

		function createRow(initialValue) {
			const select = document.createElement("select");
			select.name = fieldName;
			select.className = "llm-rotation-row";
			wireRow(select, initialValue);
			rowsContainer.appendChild(select);
		}

		function addRow(value) {
			createRow(value);
		}

		function getAvailable() {
			const selected = getSelectedValues();
			return options.filter(function (o) {
				return selected.indexOf(o.value) === -1;
			});
		}

		function updateAddUI() {
			const available = getAvailable();
			if (available.length === 0) {
				addBtn.classList.add("hidden");
				noAddHint.classList.remove("hidden");
			} else {
				addBtn.classList.remove("hidden");
				noAddHint.classList.add("hidden");
			}
		}

		function updateSaveButton() {
			if (!requireMin || !saveBtn) return;
			const count = getSelectedValues().length;
			saveBtn.disabled = count === 0;
			if (minHint) {
				if (count === 0) minHint.classList.remove("hidden");
				else minHint.classList.add("hidden");
			}
		}

		function updateAll() {
			rowsContainer.querySelectorAll("select.llm-rotation-row").forEach(function (sel) {
				rebuildSelect(sel, sel.value);
			});
			updateAddUI();
			updateSaveButton();
		}

		function renderModalCheckboxes(available) {
			modalList.replaceChildren();
			available.forEach(function (o) {
				const label = document.createElement("label");
				label.className = "confirm-checkbox";
				const cb = document.createElement("input");
				cb.type = "checkbox";
				cb.value = o.value;
				const span = document.createElement("span");
				span.textContent = o.label;
				label.appendChild(cb);
				label.appendChild(span);
				modalList.appendChild(label);
			});
		}

		let lastDialogFocus = null;

		dialog.addEventListener("close", function () {
			if (lastDialogFocus) lastDialogFocus.focus();
			lastDialogFocus = null;
		});

		function closeDialog() {
			dialog.close();
		}

		addBtn.addEventListener("click", function () {
			const available = getAvailable();
			if (available.length === 0) return;
			lastDialogFocus = addBtn;
			renderModalCheckboxes(available);
			dialog.showModal();
		});

		selectAllBtn.addEventListener("click", function () {
			modalList.querySelectorAll("input[type=checkbox]").forEach(function (cb) {
				cb.checked = true;
			});
		});

		clearSelBtn.addEventListener("click", function () {
			modalList.querySelectorAll("input[type=checkbox]").forEach(function (cb) {
				cb.checked = false;
			});
		});

		cancelBtn.addEventListener("click", function () {
			closeDialog();
		});

		confirmBtn.addEventListener("click", function () {
			const checked = [];
			modalList.querySelectorAll("input[type=checkbox]:checked").forEach(function (cb) {
				checked.push(cb.value);
			});
			checked.forEach(function (v) {
				addRow(v);
			});
			closeDialog();
			updateAll();
		});

		rowsContainer.querySelectorAll("select.llm-rotation-row").forEach(function (sel) {
			wireRow(sel, sel.dataset.initial || sel.value);
		});

		updateAll();
	}

	document.addEventListener("DOMContentLoaded", function () {
		document.querySelectorAll(".llm-rotation").forEach(init);
	});
})();
