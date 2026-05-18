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

vim.keymap.set(
	"n",
	"<CR>",
	dismiss_after([[\<Plug>fugitive:\<CR>]]),
	{ buffer = true, desc = "Open file and dismiss status" }
)

vim.keymap.set(
	"n",
	"dd",
	dismiss_after([[\<Plug>fugitive:dd]]),
	{ buffer = true, desc = "Diff file and dismiss status" }
)
