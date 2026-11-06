/** One row in a RecordPicker. Here because `<script setup>` cannot export types. */
export interface PickerOption {
  id: string
  label: string
  /** Dim second line: an ancestor path, a code, an IP. */
  sublabel?: string
  /**
   * Indent level for hierarchies. When set, `sublabel` (the ancestor path) shows
   * only while filtering; otherwise it always shows.
   */
  depth?: number
  disabled?: boolean
}
