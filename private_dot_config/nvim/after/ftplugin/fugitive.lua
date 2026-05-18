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

-- <CR> on an untracked directory would run `:Gedit mydir/`, which (a) opens a
-- fugitive directory tree-view buffer, (b) triggers snacks explorer via
-- replace_netrw on the directory BufEnter, and (c) can lcd into the dir as
-- part of tree-view setup. Bail before any of that happens.
vim.keymap.set("n", "<CR>", function()
	local info = classify_entry()
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
	if info.section == "Untracked" then
		vim.notify("No diff available for untracked " .. (info.is_dir and "directory" or "file"), vim.log.levels.INFO)
		return
	end
	dismiss_after([[\<Plug>fugitive:dd]])()
end, { buffer = true, desc = "Diff file and dismiss status" })
