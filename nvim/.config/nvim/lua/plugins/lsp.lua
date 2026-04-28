return {
	{
		"neovim/nvim-lspconfig",
		event = { "BufReadPre", "BufNewFile" },
		dependencies = {
			{ "williamboman/mason.nvim", opts = {} },
			{
				"williamboman/mason-lspconfig.nvim",
				opts = {
					automatic_enable = false,
					ensure_installed = {
						"ts_ls",
						"eslint",
						"bashls",
						"dockerls",
						"docker_compose_language_service",
						"gopls",
						"lua_ls",
					},
				},
			},
			{
				"WhoIsSethDaniel/mason-tool-installer.nvim",
				opts = {
					ensure_installed = { "prettier", "shellcheck", "shfmt", "hadolint", "stylua" },
				},
			},
			"saghen/blink.cmp",
		},
		config = function()
			local capabilities = require("blink.cmp").get_lsp_capabilities()

			vim.api.nvim_create_autocmd("LspAttach", {
				callback = function(args)
					local bufnr = args.buf
					local client = vim.lsp.get_client_by_id(args.data.client_id)

					local map = function(mode, lhs, rhs, desc)
						vim.keymap.set(mode, lhs, rhs, { buffer = bufnr, desc = desc })
					end
					map("n", "<leader>ld", vim.lsp.buf.definition, "Go to definition")
					map("n", "<leader>lD", vim.lsp.buf.declaration, "Go to declaration")
					map("n", "<leader>lr", vim.lsp.buf.references, "References")
					map("n", "<leader>li", vim.lsp.buf.implementation, "Go to implementation")
					map("n", "<leader>lk", vim.lsp.buf.hover, "Hover")
					map("n", "<leader>ln", vim.lsp.buf.rename, "Rename")
					map({ "n", "v" }, "<leader>ca", vim.lsp.buf.code_action, "Code action")
					map("n", "[d", vim.diagnostic.goto_prev, "Previous diagnostic")
					map("n", "]d", vim.diagnostic.goto_next, "Next diagnostic")

					if client and client.name == "eslint" then
						vim.api.nvim_create_autocmd("BufWritePre", {
							buffer = bufnr,
							callback = function()
								vim.lsp.buf.code_action({
									context = { only = { "source.fixAll.eslint" } },
									apply = true,
								})
							end,
						})
					end
				end,
			})

			vim.lsp.config("*", { capabilities = capabilities })
			vim.lsp.config("ts_ls", {})
			vim.lsp.config("eslint", {})
			vim.lsp.config("bashls", {})
			vim.lsp.config("dockerls", {})
			vim.lsp.config("docker_compose_language_service", {})
			vim.lsp.config("gopls", {})
			vim.lsp.config("lua_ls", {
				settings = {
					Lua = {
						workspace = { checkThirdParty = false },
						telemetry = { enable = false },
					},
				},
			})
			vim.lsp.enable({
				"ts_ls",
				"eslint",
				"bashls",
				"dockerls",
				"docker_compose_language_service",
				"gopls",
				"lua_ls",
			})
		end,
	},
}
