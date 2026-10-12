-- Where a session actually went, when that is not the target itself.
--
-- An ecs-exec connection names a service, and every connect picks whichever
-- task is newest and healthy at that moment. The target says "api"; this says
-- which task ARN the shell really ran in, which is what an audit needs once
-- that task has long been replaced. Empty for the kinds whose target already
-- is the machine.

ALTER TABLE session_log ADD COLUMN landed_on TEXT NOT NULL DEFAULT '';
