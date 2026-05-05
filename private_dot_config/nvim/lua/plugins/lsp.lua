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

			vim.diagnostic.config({
				float = { border = "rounded" },
			})

			vim.api.nvim_create_autocmd("LspAttach", {
				callback = function(args)
					local bufnr = args.buf
					local client = vim.lsp.get_client_by_id(args.data.client_id)

					local map = function(mode, lhs, rhs, desc)
						vim.keymap.set(mode, lhs, rhs, { buffer = bufnr, desc = desc })
					end
					map("n", "<leader>ld", function()
						Snacks.picker.lsp_definitions()
					end, "Go to definition")
					map("n", "<leader>lD", function()
						Snacks.picker.lsp_declarations()
					end, "Go to declaration")
					map("n", "<leader>lr", function()
						Snacks.picker.lsp_references()
					end, "References")
					map("n", "<leader>li", function()
						Snacks.picker.lsp_implementations()
					end, "Go to implementation")
					map("n", "<leader>lk", function()
						vim.lsp.buf.hover({ border = "rounded" })
					end, "Hover")
					map("n", "<leader>ln", vim.lsp.buf.rename, "Rename")
					map({ "n", "v" }, "<leader>ca", vim.lsp.buf.code_action, "Code action")
					map("n", "[d", function()
						vim.diagnostic.jump({ count = -1 })
					end, "Previous diagnostic")
					map("n", "]d", function()
						vim.diagnostic.jump({ count = 1 })
					end, "Next diagnostic")

					if client and client.name == "eslint" then
						vim.api.nvim_create_autocmd("BufWritePre", {
							buffer = bufnr,
							command = "LspEslintFixAll",
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
