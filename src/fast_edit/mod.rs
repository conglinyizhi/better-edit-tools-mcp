pub(crate) mod core;
mod edit;
mod func_range;
mod tag_range;
mod params;
mod write;

pub use edit::{op_show, op_replace, op_insert, op_delete, op_batch, ShowEnd};
pub use func_range::op_func_range;
pub use tag_range::op_tag_range;
pub use params::{CommonEditParams, ContentTarget, TargetSpan, resolve_target_span};
pub use write::op_write;
