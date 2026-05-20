-- Fallback: if the user switches buffers in the status window manually, the
-- buffer goes hidden and is wiped (instead of lingering in :ls).
vim.bo.bufhidden = "wipe"

-- Replaces fugitive's O (tabedit + focus). Invokes the original via the
-- <Plug>fugitive:O alias that s:Map registers, then restores tab/window so
-- focus stays on the status buffer. Visual mode iterates the selection.
local function open_in_bg_tab(lnum)
	local orig_tab = vim.api.nvim_get_current_tabpage()
	local orig_win = vim.api.nvim_get_current_win()
	vim.api.nvim_win_set_cursor(orig_win, { lnum, 0 })
	pcall(function()
		vim.cmd([[silent! call feedkeys("\<Plug>fugitive:O", 'x')]])
	end)
	if vim.api.nvim_tabpage_is_valid(orig_tab) then
		vim.api.nvim_set_current_tabpage(orig_tab)
	end
	if vim.api.nvim_win_is_valid(orig_win) then
		vim.api.nvim_set_current_win(orig_win)
	end
end

vim.keymap.set("n", "O", function()
	open_in_bg_tab(vim.fn.line("."))
end, { buffer = true, desc = "Open file under cursor in background tab" })

vim.keymap.set("x", "O", function()
	local s, e = vim.fn.line("v"), vim.fn.line(".")
	if s > e then
		s, e = e, s
	end
	vim.cmd("normal! \27")
	for lnum = s, e do
		open_in_bg_tab(lnum)
	end
end, { buffer = true, desc = "Open selected files in background tabs" })

-- Fugitive's :Gedit (used by <CR>) and :Gdiffsplit (dd) call s:BlurStatus
-- first, which jumps focus to another window. The status buffer is never
-- hidden, so bufhidden=wipe doesn't fire on its own — wipe it explicitly.
local function dismiss_after(plug_keys)
	return function()
		local fugitive_buf = vim.api.nvim_get_current_buf()
		pcall(function()
			vim.cmd(string.format([[silent! call feedkeys("%s", 'x')]], plug_keys))
		end)
		if not vim.api.nvim_buf_is_valid(fugitive_buf) then
			return
		end
		-- No blur happened (e.g. <CR> on a section header) — leave status open.
		if vim.api.nvim_get_current_buf() == fugitive_buf then
			return
		end
		for _, win in ipairs(vim.api.nvim_list_wins()) do
			if vim.api.nvim_win_is_valid(win) and vim.api.nvim_win_get_buf(win) == fugitive_buf then
				pcall(vim.api.nvim_win_close, win, false)
			end
		end
		pcall(vim.api.nvim_buf_delete, fugitive_buf, { force = false })
	end
end

-- Git porcelain collapses untracked directories into a single `mydir/` entry
-- (it never enumerates their contents), so an "Untracked" line that reads like
-- a file may actually be a directory. We need to detect that to guard <CR>/dd.
-- Mirrors the section/path extraction in fugitive's s:StageInfo.
local function classify_entry()
	local lnum = vim.fn.line(".")
	local line = vim.fn.getline(lnum)
	-- Hunk lines (handled natively by fugitive) — don't intercept.
	if line:match("^[ @+%-]") then
		return { section = nil }
	end
	local path = line:match("^[A-Z?] (.*)$") or line
	local section
	for n = lnum, 1, -1 do
		local heading = vim.fn.getline(n):match("^(%u%l+).- %(%d+%+?%)$")
		if heading then
			section = heading
			break
		end
	end
	local is_dir = path:sub(-1) == "/"
	if not is_dir and path ~= "" then
		local ok, abs = pcall(vim.fn.FugitiveFind, path, vim.api.nvim_get_current_buf())
		if ok and abs and abs ~= "" then
			is_dir = vim.fn.isdirectory(abs) == 1
		end
	end
	return { section = section, path = path, is_dir = is_dir }
end

-- Returns the N from `stash@{N}` on the current line, or nil if the cursor
-- isn't on a stash entry. Used by the stash maps below.
local function stash_index_at_cursor()
	local n = vim.fn.getline("."):match("^stash@{(%d+)}")
	return n and tonumber(n) or nil
end

-- Dismisses the status window/buffer the same way `dismiss_after` does, but
-- without the feedkeys indirection — used by stash-line maps that invoke
-- fugitive commands directly (`:Gedit stash@{N}`, etc).
local function dismiss_status_if_blurred(fugitive_buf)
	if not vim.api.nvim_buf_is_valid(fugitive_buf) then
		return
	end
	if vim.api.nvim_get_current_buf() == fugitive_buf then
		return
	end
	for _, win in ipairs(vim.api.nvim_list_wins()) do
		if vim.api.nvim_win_is_valid(win) and vim.api.nvim_win_get_buf(win) == fugitive_buf then
			pcall(vim.api.nvim_win_close, win, false)
		end
	end
	pcall(vim.api.nvim_buf_delete, fugitive_buf, { force = false })
end

-- <CR> on an untracked directory would run `:Gedit mydir/`, which (a) opens a
-- fugitive directory tree-view buffer, (b) triggers snacks explorer via
-- replace_netrw on the directory BufEnter, and (c) can lcd into the dir as
-- part of tree-view setup. Bail before any of that happens.
vim.keymap.set("n", "<CR>", function()
	local info = classify_entry()
	if info.section == "Stashes" then
		local idx = stash_index_at_cursor()
		if not idx then
			return
		end
		local fugitive_buf = vim.api.nvim_get_current_buf()
		pcall(vim.cmd, "Gedit stash@{" .. idx .. "}")
		dismiss_status_if_blurred(fugitive_buf)
		return
	end
	if info.section == "Untracked" and info.is_dir then
		vim.notify("Untracked directory — open via <leader>e or expand with =", vim.log.levels.INFO)
		return
	end
	dismiss_after([[\<Plug>fugitive:\<CR>]])()
end, { buffer = true, desc = "Open file and dismiss status" })

-- Untracked entries have no index version, so fugitive's diff path produces
-- an empty/erroring split; for untracked dirs it also cascades into the same
-- tree-buffer/snacks explorer mess as <CR>. Bail with a message.
vim.keymap.set("n", "dd", function()
	local info = classify_entry()
	if info.section == "Stashes" then
		local idx = stash_index_at_cursor()
		if not idx then
			return
		end
		local fugitive_buf = vim.api.nvim_get_current_buf()
		pcall(vim.cmd, "Git! stash show -p stash@{" .. idx .. "}")
		dismiss_status_if_blurred(fugitive_buf)
		return
	end
	if info.section == "Untracked" then
		vim.notify("No diff available for untracked " .. (info.is_dir and "directory" or "file"), vim.log.levels.INFO)
		return
	end
	dismiss_after([[\<Plug>fugitive:dd]])()
end, { buffer = true, desc = "Diff file and dismiss status" })

-- Drop, apply, pop the stash under the cursor. `:Git stash …` runs through
-- fugitive's command machinery, which fires FugitiveChanged and reloads the
-- status buffer (re-firing FugitiveIndex → re-renders our section).
vim.keymap.set("n", "X", function()
	local info = classify_entry()
	if info.section ~= "Stashes" then
		return
	end
	local idx = stash_index_at_cursor()
	if not idx then
		return
	end
	local choice = vim.fn.confirm("Drop stash@{" .. idx .. "}?", "&Yes\n&No", 2)
	if choice ~= 1 then
		return
	end
	vim.cmd("Git stash drop --quiet stash@{" .. idx .. "}")
end, { buffer = true, desc = "Drop stash under cursor" })

vim.keymap.set("n", "a", function()
	local info = classify_entry()
	if info.section ~= "Stashes" then
		return
	end
	local idx = stash_index_at_cursor()
	if not idx then
		return
	end
	vim.cmd("Git stash apply --quiet --index stash@{" .. idx .. "}")
end, { buffer = true, desc = "Apply stash under cursor" })

vim.keymap.set("n", "P", function()
	local info = classify_entry()
	if info.section ~= "Stashes" then
		return
	end
	local idx = stash_index_at_cursor()
	if not idx then
		return
	end
	vim.cmd("Git stash pop --quiet --index stash@{" .. idx .. "}")
end, { buffer = true, desc = "Pop stash under cursor" })

-- Append a `Stashes (N)` section to the status buffer. Fugitive doesn't render
-- one (autoload/fugitive.vim:2991-3015 only adds Untracked/Unstaged/Staged/
-- Unpushed/Unpulled), so we hook the documented `User FugitiveIndex` event
-- which fires at the end of fugitive#BufReadStatus and on every reload.
-- Heading format matches fugitive's `Name (count)` style so the existing
-- `classify_entry` heading regex picks it up unchanged.
local stash_group = vim.api.nvim_create_augroup("FugitiveStashesUI", { clear = true })
vim.api.nvim_create_autocmd("User", {
	group = stash_group,
	pattern = "FugitiveIndex",
	callback = function()
		if vim.b.fugitive_type ~= "index" then
			return
		end
		local git_dir = vim.b.git_dir
		if not git_dir or git_dir == "" then
			return
		end
		local stashes = vim.fn.systemlist({ "git", "--git-dir=" .. git_dir, "stash", "list", "--pretty=format:%gd %s" })
		if vim.v.shell_error ~= 0 then
			return
		end
		-- Filter empties (systemlist on empty output sometimes returns {""}).
		local entries = {}
		for _, line in ipairs(stashes) do
			if line ~= "" then
				table.insert(entries, line)
			end
		end
		if #entries == 0 then
			return
		end
		local bufnr = vim.api.nvim_get_current_buf()
		local was_modifiable = vim.bo[bufnr].modifiable
		vim.bo[bufnr].modifiable = true
		local payload = { "", string.format("Stashes (%d)", #entries) }
		for _, line in ipairs(entries) do
			table.insert(payload, line)
		end
		local total = vim.api.nvim_buf_line_count(bufnr)
		vim.api.nvim_buf_set_lines(bufnr, total, total, false, payload)
		vim.bo[bufnr].modifiable = was_modifiable
		vim.bo[bufnr].modified = false
	end,
})
