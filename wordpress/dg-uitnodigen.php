<?php
/**
 * Plugin Name: Digitaal GemeenGoed — collega's uitnodigen
 * Description: Op de Leden-pagina nodigt een lid collega's uit om lid te worden. De site stuurt een eenmalige mail namens het lid, met een link naar het aanmeldformulier (Keycloak). Er wordt vooraf geen account aangemaakt.
 *
 * Grenzen: hooguit DG_UITNODIGEN_PER_DAG per lid per dag; hetzelfde adres niet
 * opnieuw binnen DG_UITNODIGING_DAGEN dagen; wie al een account op de site
 * heeft, krijgt geen uitnodiging.
 *
 * Privacy: het adres van wie (nog) geen lid is, staat in {prefix}dg_uitnodigingen
 * zolang de uitnodiging loopt (30 dagen); daarna gaat de rij weg. Wordt iemand
 * lid, dan blijft alleen vastgelegd wie wie uitnodigde (voor de kerngroep), en
 * dat ook maar een jaar. De persoonlijke tekst wordt niet bewaard.
 *
 * Afscherming (zolang de site een wachtwoord heeft, caddy/Caddyfile): Caddy laat
 * /uitnodiging/<sleutel> door. Is de sleutel geldig, dan zet de site het
 * cookie van de afscherming (DG_TOEGANG_COOKIE) en stuurt door naar het
 * aanmelden; zonder geldige sleutel blijft alles dicht.
 *
 * @package DigitaalGemeenGoed
 */

defined( 'ABSPATH' ) || exit;

const DG_UIT_DB_VERSIE      = 1;
const DG_UITNODIGEN_PER_DAG = 10;
const DG_UITNODIGING_DAGEN  = 30;

function dg_uit_tabel(): string {
	global $wpdb;
	return $wpdb->prefix . 'dg_uitnodigingen';
}

function dg_uit_installeer(): void {
	global $wpdb;
	require_once ABSPATH . 'wp-admin/includes/upgrade.php';
	$tabel = dg_uit_tabel();
	dbDelta(
		"CREATE TABLE $tabel (
			id bigint(20) unsigned NOT NULL AUTO_INCREMENT,
			email varchar(190) NOT NULL DEFAULT '',
			door bigint(20) unsigned NOT NULL,
			sleutel char(64) NOT NULL,
			aangemaakt datetime NOT NULL,
			verloopt datetime NOT NULL,
			status varchar(20) NOT NULL DEFAULT 'uitgenodigd',
			lid bigint(20) unsigned NOT NULL DEFAULT 0,
			lid_op datetime DEFAULT NULL,
			PRIMARY KEY  (id),
			KEY email (email),
			KEY door (door),
			UNIQUE KEY sleutel (sleutel)
		) {$wpdb->get_charset_collate()};"
	);
	update_option( 'dg_uit_db', DG_UIT_DB_VERSIE );
}

add_action(
	'init',
	static function () {
		if ( wp_installing() || ! is_blog_installed() ) {
			return;
		}
		if ( (int) get_option( 'dg_uit_db' ) !== DG_UIT_DB_VERSIE ) {
			dg_uit_installeer();
		}
		if ( ! wp_next_scheduled( 'dg_uit_opruimen' ) ) {
			wp_schedule_event( time() + HOUR_IN_SECONDS, 'daily', 'dg_uit_opruimen' );
		}
	}
);

// Opruimen: verlopen uitnodigingen weg; van wie lid werd, na een jaar.
add_action(
	'dg_uit_opruimen',
	static function () {
		global $wpdb;
		$tabel = dg_uit_tabel();
		$nu    = current_time( 'mysql', true );
		$wpdb->query( $wpdb->prepare( "DELETE FROM $tabel WHERE status = 'uitgenodigd' AND verloopt < %s", $nu ) ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
		$wpdb->query( $wpdb->prepare( "DELETE FROM $tabel WHERE status = 'lid' AND lid_op < %s", gmdate( 'Y-m-d H:i:s', time() - YEAR_IN_SECONDS ) ) ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
	}
);

// Wie inlogt en uitgenodigd was: lid geworden (het adres is dan niet meer nodig).
function dg_uit_lid_geworden( $user ): void {
	if ( ! $user instanceof WP_User || '' === $user->user_email ) {
		return;
	}
	global $wpdb;
	$tabel = dg_uit_tabel();
	$wpdb->query( // phpcs:ignore WordPress.DB.DirectDatabaseQuery
		$wpdb->prepare(
			"UPDATE $tabel SET status = 'lid', lid = %d, lid_op = %s, email = '' WHERE status = 'uitgenodigd' AND email = %s", // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
			$user->ID,
			current_time( 'mysql', true ),
			strtolower( $user->user_email )
		)
	);
}
add_action( 'openid-connect-generic-user-create', 'dg_uit_lid_geworden', 20 );
add_action( 'openid-connect-generic-update-user-using-current-claim', 'dg_uit_lid_geworden', 20 );

// ---------------------------------------------------------------------------
// Op de Leden-pagina
// ---------------------------------------------------------------------------

add_action( 'bp_before_directory_members_page', static fn() => print dg_uit_blok() ); // phpcs:ignore WordPress.Security.EscapeOutput
add_shortcode( 'dg_uitnodigen', 'dg_uit_blok' );

function dg_uit_blok(): string {
	if ( ! is_user_logged_in() ) {
		return '';
	}
	global $wpdb;
	$uid   = get_current_user_id();
	$tabel = dg_uit_tabel();
	// phpcs:ignore WordPress.Security.NonceVerification.Recommended -- alleen een melding.
	$soort = sanitize_key( wp_unslash( $_GET['dg_uit'] ?? '' ) );
	$open  = '' !== $soort;
	$h     = '<details class="dg-uit" id="uitnodigen"' . ( $open ? ' open' : '' ) . '><summary><span class="dg-icoon dg-icoon-mail" aria-hidden="true"></span>Collega uitnodigen</summary><div class="dg-uit-inhoud">';
	$h    .= dg_uit_melding( $soort );
	$h    .= '<p>Ken je iemand die mee zou willen doen? Nodig je collega uit. De site stuurt een mail namens jou, met een link om lid te worden. Antwoorden op die mail komt bij jou.</p>'
		. '<form method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '">'
		. wp_nonce_field( 'dg_uitnodigen', '_wpnonce', true, false )
		. '<input type="hidden" name="action" value="dg_uitnodigen">'
		. '<label>E-mailadres van je collega <small>(meer adressen: gescheiden door komma\'s, hooguit 5)</small><input type="text" name="adressen" required inputmode="email" autocomplete="off" placeholder="naam@gemeente.nl"></label>'
		. '<label>Persoonlijk bericht <small>(optioneel)</small><textarea name="bericht" rows="3" maxlength="1000" placeholder="Bijvoorbeeld: hier delen gemeenten hun ervaringen met de Office Assistent. Iets voor jou?"></textarea></label>'
		. '<p class="dg-uit-klein">Je collega krijgt één mail. Reageert hij of zij niet, dan volgt er niets en verwijderen we het adres na ' . (int) DG_UITNODIGING_DAGEN . ' dagen. Hooguit ' . (int) DG_UITNODIGEN_PER_DAG . ' uitnodigingen per dag.</p>'
		. '<button type="submit" class="dg-db-knop">Uitnodiging versturen</button>'
		. '</form>';
	$rijen = (array) $wpdb->get_results( $wpdb->prepare( "SELECT email, status, aangemaakt, verloopt, lid FROM $tabel WHERE door = %d ORDER BY aangemaakt DESC LIMIT 20", $uid ), ARRAY_A ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
	if ( $rijen ) {
		$h .= '<h3>Jouw uitnodigingen</h3><ul class="dg-uit-lijst">';
		foreach ( $rijen as $r ) {
			if ( 'lid' === $r['status'] ) {
				$wie = get_userdata( (int) $r['lid'] );
				$h  .= '<li><span>' . esc_html( $wie ? $wie->display_name : 'Lid geworden' ) . '</span><span class="dg-db-status dg-db-status-actief">Lid geworden</span></li>';
			} else {
				$h .= '<li><span>' . esc_html( $r['email'] ) . '</span><span class="dg-db-status dg-db-status-in-verkenning">Uitgenodigd ' . esc_html( wp_date( 'j M', strtotime( $r['aangemaakt'] . ' UTC' ) ) ) . '</span></li>';
			}
		}
		$h .= '</ul>';
	}
	return $h . '</div></details>';
}

function dg_uit_melding( string $soort ): string {
	if ( '' === $soort ) {
		return '';
	}
	$uitslag = get_transient( 'dg_uit_uitslag_' . get_current_user_id() );
	delete_transient( 'dg_uit_uitslag_' . get_current_user_id() );
	$regels = is_array( $uitslag ) ? $uitslag : array();
	$fout   = 'fout' === $soort;
	if ( ! $regels ) {
		$regels = array( $fout ? 'Dat lukte niet. Probeer het opnieuw.' : 'Verstuurd.' );
	}
	return '<div class="dg-cf-melding' . ( $fout ? ' dg-cf-melding-fout' : '' ) . '" role="status"><ul>' . implode( '', array_map( static fn( $r ) => '<li>' . esc_html( $r ) . '</li>', $regels ) ) . '</ul></div>';
}

// ---------------------------------------------------------------------------
// Versturen
// ---------------------------------------------------------------------------

add_action(
	'admin_post_dg_uitnodigen',
	static function () {
		$uid   = get_current_user_id();
		$terug = remove_query_arg( 'dg_uit', wp_get_referer() ?: home_url( '/leden/' ) );
		$naar  = static function ( string $soort, array $regels ) use ( $uid, $terug ): void {
			set_transient( 'dg_uit_uitslag_' . $uid, $regels, 10 * MINUTE_IN_SECONDS );
			wp_safe_redirect( add_query_arg( 'dg_uit', $soort, strtok( $terug, '#' ) ) . '#uitnodigen' );
			exit;
		};
		if ( ! $uid || ! check_admin_referer( 'dg_uitnodigen' ) ) {
			$naar( 'fout', array( 'Verlopen formulier. Ververs de pagina en probeer het opnieuw.' ) );
		}
		$bericht  = mb_substr( trim( sanitize_textarea_field( wp_unslash( $_POST['bericht'] ?? '' ) ) ), 0, 1000 );
		$adressen = array_unique( array_filter( array_map( static fn( $a ) => strtolower( trim( $a ) ), preg_split( '/[,;\s]+/', (string) wp_unslash( $_POST['adressen'] ?? '' ) ) ) ) );
		if ( ! $adressen ) {
			$naar( 'fout', array( 'Vul een e-mailadres in.' ) );
		}
		if ( count( $adressen ) > 5 ) {
			$naar( 'fout', array( 'Hooguit 5 adressen tegelijk.' ) );
		}
		global $wpdb;
		$tabel   = dg_uit_tabel();
		$vandaag = (int) $wpdb->get_var( $wpdb->prepare( "SELECT COUNT(*) FROM $tabel WHERE door = %d AND aangemaakt > %s", $uid, gmdate( 'Y-m-d H:i:s', time() - DAY_IN_SECONDS ) ) ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
		$regels  = array();
		$gelukt  = 0;
		foreach ( $adressen as $adres ) {
			if ( ! is_email( $adres ) ) {
				$regels[] = "$adres: geen geldig e-mailadres.";
				continue;
			}
			if ( get_user_by( 'email', $adres ) ) {
				$regels[] = "$adres is al lid.";
				continue;
			}
			$loopt = (int) $wpdb->get_var( $wpdb->prepare( "SELECT COUNT(*) FROM $tabel WHERE email = %s AND status = 'uitgenodigd' AND verloopt > %s", $adres, current_time( 'mysql', true ) ) ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
			if ( $loopt ) {
				$regels[] = "$adres is de afgelopen " . DG_UITNODIGING_DAGEN . ' dagen al uitgenodigd.';
				continue;
			}
			if ( $vandaag >= DG_UITNODIGEN_PER_DAG ) {
				$regels[] = "$adres: niet verstuurd, je hebt vandaag al " . DG_UITNODIGEN_PER_DAG . ' uitnodigingen verstuurd. Morgen kan het weer.';
				continue;
			}
			$sleutel = bin2hex( random_bytes( 24 ) );
			$nu      = time();
			$wpdb->insert(
				$tabel,
				array(
					'email'      => $adres,
					'door'       => $uid,
					'sleutel'    => hash( 'sha256', $sleutel ),
					'aangemaakt' => gmdate( 'Y-m-d H:i:s', $nu ),
					'verloopt'   => gmdate( 'Y-m-d H:i:s', $nu + DG_UITNODIGING_DAGEN * DAY_IN_SECONDS ),
				)
			);
			if ( dg_uit_mail( $adres, $sleutel, $bericht, wp_get_current_user() ) ) {
				++$vandaag;
				++$gelukt;
				$regels[] = "Uitnodiging verstuurd naar $adres.";
			} else {
				$wpdb->delete( $tabel, array( 'id' => $wpdb->insert_id ) );
				$regels[] = "$adres: de mail kon niet worden verstuurd. Probeer het later opnieuw.";
			}
		}
		$naar( $gelukt ? 'ok' : 'fout', $regels );
	}
);

function dg_uit_mail( string $adres, string $sleutel, string $bericht, WP_User $door ): bool {
	$link = home_url( '/uitnodiging/' . $sleutel );
	$naam = $door->display_name ?: $door->user_login;
	$org  = (string) get_user_meta( $door->ID, 'dg_organisatie', true );
	$wie  = $naam . ( $org ? " ($org)" : '' );
	$site = 'Digitaal GemeenGoed';

	$tekst = "Hallo,\n\n$wie nodigt je uit om lid te worden van $site: een community van gemeenten en andere overheden die samen open source oplossingen ontwikkelen en kennis delen.\n\n";
	if ( '' !== $bericht ) {
		$tekst .= "$naam schrijft:\n\n" . preg_replace( '/^/m', '> ', $bericht ) . "\n\n";
	}
	$tekst .= "Lid worden is gratis. Met deze link maak je een account aan:\n\n$link\n\n"
		. 'De link is ' . DG_UITNODIGING_DAGEN . " dagen geldig. Gebruik bij voorkeur het e-mailadres van je werk: dan word je vanzelf aan je organisatie gekoppeld.\n\n"
		. "Wil je niet? Dan hoef je niets te doen. Je krijgt geen herinnering en we verwijderen je adres na " . DG_UITNODIGING_DAGEN . " dagen. Vragen over deze uitnodiging? Antwoord op deze mail; die gaat naar $naam.\n\n"
		. "Met vriendelijke groet,\n$site\n" . home_url( '/' ) . "\n";

	$koppen = array( 'Content-Type: text/plain; charset=UTF-8' );
	if ( is_email( $door->user_email ) ) {
		$koppen[] = 'Reply-To: ' . str_replace( array( "\r", "\n", '"' ), '', $naam ) . ' <' . $door->user_email . '>';
	}
	return (bool) wp_mail( $adres, "$naam nodigt je uit voor $site", $tekst, $koppen );
}

// ---------------------------------------------------------------------------
// De link in de mail: /uitnodiging/<sleutel>
// ---------------------------------------------------------------------------

add_action(
	'template_redirect',
	static function () {
		$pad = trim( (string) wp_parse_url( $_SERVER['REQUEST_URI'] ?? '', PHP_URL_PATH ), '/' );
		if ( ! preg_match( '#^uitnodiging/([0-9a-f]{48})$#', $pad, $m ) ) {
			if ( str_starts_with( $pad, 'uitnodiging/' ) ) {
				dg_uit_niet_geldig();
			}
			return;
		}
		global $wpdb;
		$tabel = dg_uit_tabel();
		$rij   = $wpdb->get_row( $wpdb->prepare( "SELECT id, status, verloopt FROM $tabel WHERE sleutel = %s", hash( 'sha256', $m[1] ) ), ARRAY_A ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared
		if ( ! $rij || strtotime( $rij['verloopt'] . ' UTC' ) < time() ) {
			dg_uit_niet_geldig();
		}
		// Langs de afscherming (als die aan staat): hetzelfde cookie als na het wachtwoord.
		$cookie = (string) getenv( 'DG_TOEGANG_COOKIE' );
		if ( '' !== $cookie ) {
			setcookie(
				'dg_toegang',
				$cookie,
				array(
					'expires'  => time() + 30 * DAY_IN_SECONDS,
					'path'     => '/',
					'domain'   => (string) wp_parse_url( home_url(), PHP_URL_HOST ),
					'secure'   => is_ssl(),
					'httponly' => true,
					'samesite' => 'Lax',
				)
			);
		}
		nocache_headers();
		wp_safe_redirect( is_user_logged_in() ? home_url( '/' ) : home_url( '/lid-worden/' ) );
		exit;
	},
	-1
);

/** Een kale pagina: zonder geldige uitnodiging niets van de site laten zien. */
function dg_uit_niet_geldig(): void {
	nocache_headers();
	status_header( 410 );
	header( 'Content-Type: text/html; charset=utf-8' );
	echo '<!doctype html><html lang="nl"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Uitnodiging verlopen · Digitaal GemeenGoed</title>'
		. '<body style="font-family:system-ui,sans-serif;max-width:34rem;margin:4rem auto;padding:0 16px;color:#2b3a4e;line-height:1.5">'
		. '<h1 style="color:#0a2654;font-size:1.4rem">Deze uitnodiging is niet (meer) geldig</h1>'
		. '<p>Een uitnodiging is ' . (int) DG_UITNODIGING_DAGEN . ' dagen geldig. Vraag de collega die je uitnodigde om een nieuwe.</p></body></html>';
	exit;
}
