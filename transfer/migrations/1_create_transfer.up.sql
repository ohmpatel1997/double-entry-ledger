
CREATE TABLE transfers (
                            id uuid NOT NULL,
                            debit_account_id integer NOT NULL,
                            credit_account_id integer NOT NULL,
                            amount bigint NOT NULL,
                            created_at timestamp with time zone NOT NULL DEFAULT now(),
                            transfer_progress varchar NOT NULL DEFAULT 'initiated',
                            PRIMARY KEY (id)
);

create index if not exists index_debit_account_id_amount_transfer_state on transfers (debit_account_id, amount, transfer_progress);
create index if not exists index_credit_account_id_amount_transfer_state on transfers (credit_account_id, amount, transfer_progress);
