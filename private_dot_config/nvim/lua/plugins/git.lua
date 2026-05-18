return {
	{
		"tpope/vim-fugitive",
		dependencies = { "tpope/vim-rhubarb" },
		-- Visual-mode mappings use `:` (not `<Cmd>`) so Vim auto-prepends the
		-- `'<,'>` range to the command. `<Cmd>` skips command-line processing
		-- and would run the command with no range, losing the selection.
		keys = {
			{
				"<leader>gs",
				function()
					for _, buf in ipairs(vim.api.nvim_list_bufs()) do
						if vim.api.nvim_buf_is_loaded(buf) and vim.bo[buf].filetype == "fugitive" then
							vim.api.nvim_buf_delete(buf, { force = false })
							return
						end
					end
					vim.cmd("Git")
				end,
				desc = "Toggle git status",
			},

			{ "<leader>gy", "<Cmd>.GBrowse!<CR>", desc = "Yank GitHub permalink" },
			{ "<leader>gy", ":GBrowse!<CR>", mode = "v", desc = "Yank GitHub permalink" },

			{ "<leader>gb", "<Cmd>GBrowse<CR>", desc = "Browse file on GitHub" },
			{ "<leader>gb", ":GBrowse<CR>", mode = "v", desc = "Browse selection on GitHub" },

			{ "<leader>gh", "<Cmd>!gh pr view --web<CR>", desc = "Open current PR on GitHub" },
			{ "<leader>gpr", "<Cmd>!gh pr create --draft --web<CR>", desc = "Open draft PR on GitHub" },
		},
	},
}
