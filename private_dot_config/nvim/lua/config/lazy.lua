local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not (vim.uv or vim.loop).fs_stat(lazypath) then
	local lazyrepo = "https://github.com/folke/lazy.nvim.git"
	local out = vim.fn.system({ "git", "clone", "--filter=blob:none", "--branch=stable", lazyrepo, lazypath })
	if vim.v.shell_error ~= 0 then
		vim.api.nvim_echo({
			{ "Failed to clone lazy.nvim:\n", "ErrorMsg" },
			{ out, "WarningMsg" },
			{ "\nPress any key to exit..." },
		}, true, {})
		vim.fn.getchar()
		os.exit(1)
	end
end
vim.opt.rtp:prepend(lazypath)

require("lazy").setup({
	spec = {
		{ import = "plugins" },
	},
	install = { colorscheme = { "catppuccin-mocha", "habamax" } },
	checker = { enabled = true },
	rocks = { enabled = false },
})

-- Keep the chezmoi source copy of lazy-lock.json in sync with the live one.
-- lazy.nvim rewrites ~/.config/nvim/lazy-lock.json on install/update/sync/clean,
-- which would otherwise make `chezmoi apply` prompt for confirmation.
if vim.fn.executable("chezmoi") == 1 then
	vim.api.nvim_create_autocmd("User", {
		pattern = { "LazyInstall", "LazyUpdate", "LazySync", "LazyClean" },
		desc = "chezmoi re-add lazy-lock.json after lazy events",
		callback = function()
			local lock = vim.fn.stdpath("config") .. "/lazy-lock.json"
			vim.system({ "chezmoi", "re-add", lock }, { text = true }, function(obj)
				if obj.code ~= 0 then
					vim.schedule(function()
						vim.notify(
							"chezmoi re-add failed (" .. obj.code .. "): " .. (obj.stderr or ""),
							vim.log.levels.WARN
						)
					end)
				end
			end)
		end,
	})
end
