(function () {
	function init(root) {
		var configEl = root.querySelector(".llm-rotation-config");
		if (!configEl) return;
		var config = JSON.parse(configEl.textContent);
		var fieldName = config.fieldName;
		var requireMin = config.requireMin;
		var options = config.options;
		var labels = config.labels;
		var rowsContainer = root.querySelector(".llm-rotation-rows");
		var addBtn = root.querySelector(".llm-rotation-add-btn");
		var noAddHint = root.querySelector(".llm-rotation-no-add");
		var minHint = root.querySelector(".llm-rotation-min");
		var dialog = root.querySelector(".llm-rotation-dialog");
		var modalList = root.querySelector(".llm-rotation-modal-list");
		var form = root.closest("form");
		var saveBtn = form ? form.querySelector("[data-llm-gated-save]") : null;

		var optMap = new Map();
		options.forEach(function (o) {
			optMap.set(o.value, o.label);
		});

		function getSelectedValues() {
			var vals = [];
			rowsContainer.querySelectorAll("select.llm-rotation-row").forEach(function (sel) {
				var v = sel.value;
				if (v && v !== "0:0") vals.push(v);
			});
			return vals;
		}

		function rebuildSelect(select, currentValue) {
			var selectedElse = getSelectedValues().filter(function (v) {
				return v !== currentValue;
			});
			select.innerHTML = "";
			var clearOpt = document.createElement("option");
			clearOpt.value = "0:0";
			clearOpt.textContent = labels.clearRow;
			select.appendChild(clearOpt);
			options.forEach(function (o) {
				if (selectedElse.indexOf(o.value) !== -1) return;
				var opt = document.createElement("option");
				opt.value = o.value;
				opt.textContent = o.label;
				select.appendChild(opt);
			});
			if (currentValue && currentValue !== "0:0") {
				select.value = currentValue;
				if (select.value !== currentValue) {
					var fallback = document.createElement("option");
					fallback.value = currentValue;
					fallback.textContent = optMap.get(currentValue) || currentValue;
					select.appendChild(fallback);
					select.value = currentValue;
				}
			}
		}

		function onRowChange(e) {
			var select = e.target;
			if (select.value === "0:0") {
				select.remove();
			}
			updateAll();
		}

		function addRow(value) {
			var select = document.createElement("select");
			select.name = fieldName;
			select.className = "llm-rotation-row";
			rebuildSelect(select, value);
			select.value = value;
			select.addEventListener("change", onRowChange);
			rowsContainer.appendChild(select);
		}

		function getAvailable() {
			var selected = getSelectedValues();
			return options.filter(function (o) {
				return selected.indexOf(o.value) === -1;
			});
		}

		function updateAddUI() {
			var available = getAvailable();
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
			var count = getSelectedValues().length;
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
			modalList.innerHTML = "";
			available.forEach(function (o) {
				var label = document.createElement("label");
				label.className = "confirm-checkbox";
				var cb = document.createElement("input");
				cb.type = "checkbox";
				cb.value = o.value;
				var span = document.createElement("span");
				span.textContent = o.label;
				label.appendChild(cb);
				label.appendChild(span);
				modalList.appendChild(label);
			});
		}

		addBtn.addEventListener("click", function () {
			var available = getAvailable();
			if (available.length === 0) return;
			renderModalCheckboxes(available);
			dialog.showModal();
		});

		root.querySelector(".llm-rotation-select-all").addEventListener("click", function () {
			modalList.querySelectorAll("input[type=checkbox]").forEach(function (cb) {
				cb.checked = true;
			});
		});

		root.querySelector(".llm-rotation-clear-sel").addEventListener("click", function () {
			modalList.querySelectorAll("input[type=checkbox]").forEach(function (cb) {
				cb.checked = false;
			});
		});

		root.querySelector(".llm-rotation-cancel").addEventListener("click", function () {
			dialog.close();
		});

		root.querySelector(".llm-rotation-confirm").addEventListener("click", function () {
			var checked = [];
			modalList.querySelectorAll("input[type=checkbox]:checked").forEach(function (cb) {
				checked.push(cb.value);
			});
			checked.forEach(function (v) {
				addRow(v);
			});
			dialog.close();
			updateAll();
		});

		rowsContainer.querySelectorAll("select.llm-rotation-row").forEach(function (sel) {
			var initial = sel.dataset.initial || sel.value;
			rebuildSelect(sel, initial);
			if (initial && initial !== "0:0") sel.value = initial;
			sel.addEventListener("change", onRowChange);
		});

		updateAll();
	}

	document.addEventListener("DOMContentLoaded", function () {
		document.querySelectorAll(".llm-rotation").forEach(init);
	});
})();
