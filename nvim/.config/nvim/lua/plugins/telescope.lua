return {
  {
    "nvim-telescope/telescope.nvim",
    cmd = "Telescope",
    dependencies = {
      "nvim-lua/plenary.nvim",
      "nvim-tree/nvim-web-devicons",
      {
        "nvim-telescope/telescope-fzf-native.nvim",
        build = "make",
      },
    },
    keys = {
      { "<leader>ff", "<cmd>Telescope find_files<cr>",                desc = "Find files" },
      { "<leader>fg", "<cmd>Telescope live_grep<cr>",                 desc = "Live grep" },
      { "<leader>fb", "<cmd>Telescope buffers<cr>",                   desc = "Buffers" },
      { "<leader>fh", "<cmd>Telescope help_tags<cr>",                 desc = "Help tags" },
      { "<leader>fr", "<cmd>Telescope oldfiles<cr>",                  desc = "Recent files" },
      { "<leader>fk", "<cmd>Telescope keymaps<cr>",                   desc = "Keymaps" },
      { "<leader>fc", "<cmd>Telescope commands<cr>",                  desc = "Commands" },
      { "<leader>fd", "<cmd>Telescope diagnostics<cr>",               desc = "Diagnostics" },
      { "<leader>fs", "<cmd>Telescope lsp_document_symbols<cr>",      desc = "Document symbols" },
      { "<leader>fS", "<cmd>Telescope lsp_workspace_symbols<cr>",     desc = "Workspace symbols" },
      { "<leader>/",  "<cmd>Telescope current_buffer_fuzzy_find<cr>", desc = "Search in buffer" },
    },
    opts = {
      defaults = {
        path_display = { "truncate" },
        mappings = {
          i = { ["<C-u>"] = false, ["<C-d>"] = false },
        },
      },
      extensions = {
        fzf = {
          fuzzy = true,
          override_generic_sorter = true,
          override_file_sorter = true,
          case_mode = "smart_case",
        },
      },
    },
    config = function(_, opts)
      local telescope = require("telescope")
      telescope.setup(opts)
      telescope.load_extension("fzf")
    end,
  },
}
