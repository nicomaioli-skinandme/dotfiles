return {
	{
		"tpope/vim-dadbod",
		cmd = { "DB" },
	},
	{
		"kristijanhusak/vim-dadbod-ui",
		dependencies = { "tpope/vim-dadbod" },
		cmd = {
			"DBUI",
			"DBUIToggle",
			"DBUIAddConnection",
			"DBUIFindBuffer",
			"DBUIRenameBuffer",
			"DBUILastQueryInfo",
		},
		init = function()
			vim.g.db_ui_use_nerd_fonts = 1
			vim.g.db_ui_show_database_icon = 1
			vim.g.db_ui_win_position = "left"
			vim.g.db_ui_use_nvim_notify = 1
		end,
		keys = {
			{ "<leader>Du", "<Cmd>DBUIToggle<CR>", desc = "Toggle DB UI" },
			{ "<leader>Df", "<Cmd>DBUIFindBuffer<CR>", desc = "Find DB buffer" },
			{ "<leader>Dr", "<Cmd>DBUIRenameBuffer<CR>", desc = "Rename DB buffer" },
			{ "<leader>Dq", "<Cmd>DBUILastQueryInfo<CR>", desc = "Last query info" },
		},
	},
	{
		"kristijanhusak/vim-dadbod-completion",
		dependencies = { "tpope/vim-dadbod" },
		ft = { "sql", "mysql", "plsql" },
	},
}
