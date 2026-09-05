// ui/src/types/picker.ts

/**
 * One row in a RecordPicker.
 *
 * Lives here rather than in the component because `<script setup>` cannot export
 * types, and every view that builds an options list needs to name this.
 */
export interface PickerOption {
  id: string
  label: string
  /** Dim second line: an ancestor path, a code, an IP. */
  sublabel?: string
  /**
   * Indent level for hierarchies. Its presence changes how `sublabel` is shown:
   * a hierarchical row carries its ancestors as indentation while browsing and
   * as a path while filtering, and shows exactly one at a time. A flat row's
   * sublabel is an independent fact and always shows.
   */
  depth?: number
  disabled?: boolean
}
