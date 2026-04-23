return {
  {
    "lewis6991/gitsigns.nvim",
    event = { "BufReadPre", "BufNewFile" },
    opts = {
      on_attach = function(bufnr)
        local gs = require("gitsigns")
        local function map(mode, l, r, desc)
          vim.keymap.set(mode, l, r, { buffer = bufnr, desc = desc })
        end
        map("n", "]h", function() gs.nav_hunk("next") end, "Next git hunk")
        map("n", "[h", function() gs.nav_hunk("prev") end, "Prev git hunk")
        map("n", "<leader>gh", gs.preview_hunk, "Preview hunk")
        map("n", "<leader>gb", function() gs.blame_line({ full = true }) end, "Blame line")
        map({ "n", "v" }, "<leader>gS", ":Gitsigns stage_hunk<cr>", "Stage hunk")
        map({ "n", "v" }, "<leader>gR", ":Gitsigns reset_hunk<cr>", "Reset hunk")
        map("n", "<leader>gu", gs.undo_stage_hunk, "Undo stage hunk")
      end,
    },
  },
  {
    "NeogitOrg/neogit",
    dependencies = {
      "nvim-lua/plenary.nvim",
      "nvim-telescope/telescope.nvim",
    },
    cmd = "Neogit",
    keys = {
      { "<leader>gs", "<cmd>Neogit<cr>",        desc = "Neogit status" },
      { "<leader>gc", "<cmd>Neogit commit<cr>", desc = "Neogit commit" },
      { "<leader>gp", "<cmd>Neogit pull<cr>",   desc = "Neogit pull" },
      { "<leader>gP", "<cmd>Neogit push<cr>",   desc = "Neogit push" },
    },
    opts = {
      integrations = {
        telescope = true,
      },
    },
  },
  {
    "esmuellert/codediff.nvim",
    cmd = "CodeDiff",
    keys = {
      { "<leader>gd", "<cmd>CodeDiff origin/master HEAD<cr>", desc = "Diff branch vs master" },
      { "<leader>gD", "<cmd>CodeDiff<cr>",                    desc = "Diff working tree" },
    },
  },
}
